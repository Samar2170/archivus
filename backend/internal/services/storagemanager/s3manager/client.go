package s3manager

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type Client struct {
	s3         *s3.Client
	presign    *s3.PresignClient
	BucketName string
}

func New(accountID, accessKey, secretKey, bucketName string) (*Client, error) {
	s3Client, err := newS3Client(accountID, accessKey, secretKey)
	if err != nil {
		return nil, err
	}
	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("access key and secret key must be provided")
	}
	return &Client{
		s3:         s3Client,
		presign:    s3.NewPresignClient(s3Client),
		BucketName: bucketName,
	}, nil
}

func newS3Client(accountID, accessKey, secretKey string) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("https://" + accountID + ".r2.cloudflarestorage.com")
		o.UsePathStyle = true
	}), nil
}

func (c *Client) CreateBucket(ctx context.Context, bucket string) error {
	_, err := c.s3.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	return err
}

func (c *Client) DeleteBucket(ctx context.Context, bucket string) error {
	_, err := c.s3.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucket),
	})
	return err
}

func (c *Client) CreateDirectory(ctx context.Context, bucket, dir string) error {
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(dir),
	})
	return err
}

func (c *Client) GetObject(ctx context.Context, bucket, key string) (*s3.GetObjectOutput, error) {
	return c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
}

func (c *Client) PutObject(ctx context.Context, bucket, key, contentType string, size int64, body io.Reader) error {
	input := &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	_, err := c.s3.PutObject(ctx, input)
	return err
}

func (c *Client) PutObjectBytes(ctx context.Context, bucket, key, contentType string, data []byte) error {
	return c.PutObject(ctx, bucket, key, contentType, int64(len(data)), bytes.NewReader(data))
}

func (c *Client) HeadObject(ctx context.Context, bucket, key string) (*s3.HeadObjectOutput, error) {
	return c.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
}

func (c *Client) CopyObject(ctx context.Context, bucket, srcKey, dstKey string) error {
	_, err := c.s3.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		CopySource: aws.String(bucket + "/" + srcKey),
		Key:        aws.String(dstKey),
	})
	return err
}

func (c *Client) DeleteObject(ctx context.Context, bucket, key string) error {
	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

// DeleteObjects deletes up to 1000 objects in a single request.
func (c *Client) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	objects := make([]types.ObjectIdentifier, len(keys))
	for i, k := range keys {
		objects[i] = types.ObjectIdentifier{Key: aws.String(k)}
	}
	_, err := c.s3.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{Objects: objects},
	})
	return err
}

func (c *Client) ListObjects(ctx context.Context, bucket, prefix string) ([]string, error) {
	paginator := s3.NewListObjectsV2Paginator(c.s3, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	var keys []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			keys = append(keys, aws.ToString(obj.Key))
		}
	}
	return keys, nil
}

func (c *Client) PresignGetObject(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	req, err := c.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

type S3Entry struct {
	Key   string
	IsDir bool
}

// ListObjectsOnelevel lists only the immediate children of prefix using a "/" delimiter,
// returning both objects (files) and common prefixes (virtual directories).
func (c *Client) ListObjectsOnelevel(ctx context.Context, bucket, prefix string) ([]S3Entry, error) {
	paginator := s3.NewListObjectsV2Paginator(c.s3, &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	})
	var entries []S3Entry
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			if key == prefix {
				continue // skip the directory marker itself
			}
			entries = append(entries, S3Entry{Key: key, IsDir: false})
		}
		for _, cp := range page.CommonPrefixes {
			entries = append(entries, S3Entry{Key: aws.ToString(cp.Prefix), IsDir: true})
		}
	}
	return entries, nil
}
