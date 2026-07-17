package thumbnail

import (
	archivus_constants "archivus/internal/constants"
	"archivus/internal/services/storagemanager/s3manager"
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	maxThumbWidth  = 256
	maxThumbHeight = 256
)

type Service struct {
	s3     *s3manager.Client
	bucket string

	home         string // Config.ArchivusHome, used to compute relative thumbnail path
	thumbnailDir string // Config.ThumbnailDir

	s3Enabled bool
}

func NewS3Service(s3Client *s3manager.Client, bucket string) *Service {
	return &Service{s3: s3Client, bucket: bucket, s3Enabled: true}
}

func NewDiskService(home, thumbnailDir string) *Service {
	return &Service{home: home, thumbnailDir: thumbnailDir}
}

// GenerateThumbnail generates a JPEG thumbnail for the image at pathKey and
// stores it in the backend-appropriate thumbnail location. Returns the thumbnail
// key (S3) or absolute path (disk). Returns an error if the file is not a
// supported image (JPEG, PNG, GIF).
func (s *Service) GenerateThumbnail(ctx context.Context, pathKey string) (string, error) {
	r, err := s.openReader(ctx, pathKey)
	if err != nil {
		return "", err
	}
	defer r.Close()

	src, _, err := image.Decode(r)
	if err != nil {
		return "", fmt.Errorf("not a supported image %q: %w", pathKey, err)
	}

	thumb := downscale(src, maxThumbWidth, maxThumbHeight)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 80}); err != nil {
		return "", fmt.Errorf("encode thumbnail: %w", err)
	}

	return s.writeThumb(ctx, pathKey, buf.Bytes())
}

func (s *Service) openReader(ctx context.Context, pathKey string) (io.ReadCloser, error) {
	if s.s3Enabled {
		obj, err := s.s3.GetObject(ctx, s.bucket, pathKey)
		if err != nil {
			return nil, fmt.Errorf("get s3 object %q: %w", pathKey, err)
		}
		return obj.Body, nil
	}
	f, err := os.Open(pathKey)
	if err != nil {
		return nil, fmt.Errorf("open file %q: %w", pathKey, err)
	}
	return f, nil
}

func (s *Service) writeThumb(ctx context.Context, pathKey string, data []byte) (string, error) {
	if s.s3Enabled {
		thumbKey := s3ThumbnailKey(pathKey)
		if err := s.s3.PutObjectBytes(ctx, s.bucket, thumbKey, "image/jpeg", data); err != nil {
			return "", fmt.Errorf("upload thumbnail %q: %w", thumbKey, err)
		}
		return thumbKey, nil
	}

	thumbPath := localThumbnailPath(s.home, s.thumbnailDir, pathKey)
	if err := os.MkdirAll(filepath.Dir(thumbPath), 0755); err != nil {
		return "", fmt.Errorf("create thumbnail dir for %q: %w", thumbPath, err)
	}
	if err := os.WriteFile(thumbPath, data, 0644); err != nil {
		return "", fmt.Errorf("write thumbnail %q: %w", thumbPath, err)
	}
	return thumbPath, nil
}

// s3ThumbnailKey maps an S3 source key to its thumbnail key under the thumbnails prefix.
// e.g. "drive-slug/subdir/photo.png" → ".thumbnails/drive-slug/subdir/photo.jpg"
func s3ThumbnailKey(pathKey string) string {
	dir := path.Dir(pathKey)
	name := strings.TrimSuffix(path.Base(pathKey), path.Ext(pathKey))
	if dir == "." {
		return path.Join(archivus_constants.ThumbnailDirName, name+".jpg")
	}
	return path.Join(archivus_constants.ThumbnailDirName, dir, name+".jpg")
}

// localThumbnailPath maps an absolute file path to its thumbnail path under thumbnailDir.
// e.g. "/home/user/archivus/drive/subdir/photo.png" → "/home/user/archivus/.thumbnails/drive/subdir/photo.jpg"
func localThumbnailPath(home, thumbnailDir, pathKey string) string {
	rel := strings.TrimPrefix(pathKey, home+string(filepath.Separator))
	name := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	dir := filepath.Dir(rel)
	if dir == "." {
		return filepath.Join(thumbnailDir, name+".jpg")
	}
	return filepath.Join(thumbnailDir, dir, name+".jpg")
}

// downscale returns a proportionally-downscaled copy of src that fits within
// maxW×maxH. If the image already fits, it is returned unchanged. Uses
// nearest-neighbour sampling, which is sufficient for thumbnail generation.
func downscale(src image.Image, maxW, maxH int) image.Image {
	b := src.Bounds()
	srcW, srcH := b.Dx(), b.Dy()

	scaleW := float64(maxW) / float64(srcW)
	scaleH := float64(maxH) / float64(srcH)
	scale := scaleW
	if scaleH < scaleW {
		scale = scaleH
	}
	if scale >= 1 {
		return src
	}

	dstW := int(float64(srcW) * scale)
	dstH := int(float64(srcH) * scale)
	dst := image.NewNRGBA(image.Rect(0, 0, dstW, dstH))

	for y := range dstH {
		srcY := b.Min.Y + int(float64(y)/scale)
		for x := range dstW {
			srcX := b.Min.X + int(float64(x)/scale)
			r, g, bv, a := src.At(srcX, srcY).RGBA()
			if a == 0 {
				dst.SetNRGBA(x, y, color.NRGBA{})
				continue
			}
			dst.SetNRGBA(x, y, color.NRGBA{
				R: uint8(r * 0xff / a),
				G: uint8(g * 0xff / a),
				B: uint8(bv * 0xff / a),
				A: uint8(a >> 8),
			})
		}
	}
	return dst
}
