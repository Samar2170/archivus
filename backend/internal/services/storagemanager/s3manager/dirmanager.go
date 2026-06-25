package s3manager

import (
	"archivus/internal/services/storagemanager/base"
	"archivus/internal/store"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

type S3Manager struct {
	base.BaseManager
	Client *Client
}

func GetS3Manager(s *store.Store, accountID, accessKey, secretKey, bucketName string) (*S3Manager, error) {
	client, err := New(accountID, accessKey, secretKey, bucketName)
	if err != nil {
		return nil, err
	}
	return &S3Manager{BaseManager: base.BaseManager{Store: s}, Client: client}, nil
}

func (s *S3Manager) CreateDriveDir(driveName string) (string, error) {
	return driveName, nil
}

// DeleteDriveDir deletes a drive bucket by name. Only useful as error-path cleanup
// immediately after CreateDriveDir, since the slug is re-derived and won't match
// an existing bucket created in a previous call.
func (s *S3Manager) DeleteDriveDir(driveName string) error {
	return nil
}

func (s *S3Manager) CreateDir(subFolder, driveId, userId string) error {
	if subFolder == "" {
		return errors.New("subFolder cannot be empty")
	}
	hasAccess, err := s.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("user does not have write access to this drive")
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	// trailing slash is the S3 convention for virtual directories; key is namespaced by drive slug
	key := drive.Slug + "/" + strings.Trim(subFolder, "/") + "/"
	if err := s.Client.CreateDirectory(context.Background(), s.Client.BucketName, key); err != nil {
		return fmt.Errorf("s3manager: create directory %q in bucket %q: %w", key, s.Client.BucketName, err)
	}
	relPath := filepath.Join(drive.Slug, subFolder)
	absPath := fmt.Sprintf("s3://%s/%s", s.Client.BucketName, key)
	parts := strings.Split(strings.Trim(subFolder, "/"), "/")
	name := parts[len(parts)-1]
	_, err = s.Store.CreateDirectoryMetadata(name, absPath, relPath, drive.ID.String())
	if err != nil {
		_ = s.Client.DeleteObject(context.Background(), s.Client.BucketName, key)
		return fmt.Errorf("s3manager: save directory metadata for %q: %w", key, err)
	}
	return nil
}

func (s *S3Manager) DeleteDir(relPath, driveId, userId string) error {
	hasAccess, err := s.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("user does not have write access to this drive")
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	prefix := drive.Slug + "/" + strings.Trim(relPath, "/")
	ctx := context.Background()
	keys, err := s.Client.ListObjects(ctx, s.Client.BucketName, prefix)
	if err != nil {
		return fmt.Errorf("s3manager: list prefix %q: %w", prefix, err)
	}
	if len(keys) > 0 {
		if err := s.Client.DeleteObjects(ctx, s.Client.BucketName, keys); err != nil {
			return fmt.Errorf("s3manager: delete prefix %q: %w", prefix, err)
		}
	}
	return s.Store.DeleteDirectoryMetadataByRelPath(relPath)
}

func (s *S3Manager) CreateDirV2(subFolder, driveId, userId string) error {
	if subFolder == "" {
		return errors.New("subFolder cannot be empty")
	}
	hasAccess, err := s.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("user does not have write access to this drive")
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	trimmed := strings.Trim(subFolder, "/")
	// pathKey is the S3 key for this directory (trailing slash = S3 virtual dir convention).
	pathKey := drive.Slug + "/" + trimmed + "/"
	// prefix is the parent directory's key.
	parent := filepath.Dir(trimmed)
	var prefix string
	if parent == "." {
		prefix = drive.Slug + "/"
	} else {
		prefix = drive.Slug + "/" + parent + "/"
	}
	if err := s.Client.CreateDirectory(context.Background(), s.Client.BucketName, pathKey); err != nil {
		return fmt.Errorf("s3manager: create directory %q in bucket %q: %w", pathKey, s.Client.BucketName, err)
	}
	name := filepath.Base(trimmed)
	_, err = s.Store.CreateDirectoryMetadataV2(name, pathKey, prefix, drive.ID.String())
	if err != nil {
		_ = s.Client.DeleteObject(context.Background(), s.Client.BucketName, pathKey)
		return fmt.Errorf("s3manager: save directory metadata for %q: %w", pathKey, err)
	}
	return nil
}

// s3Key converts a DB relPath (drive.Slug/subpath) to an S3 object key (subpath).
func s3Key(relPath, driveSlug string) string {
	return strings.TrimPrefix(relPath, driveSlug+"/")
}
