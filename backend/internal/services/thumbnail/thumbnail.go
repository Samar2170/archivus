package thumbnail

import (
	"archivus/internal/services/storagemanager/s3manager"
	"archivus/internal/store"
	"archivus/pkg/logging"
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
	"path/filepath"
	"strings"

	"github.com/rwcarlsen/goexif/exif"
)

const (
	maxThumbWidth  = 256
	maxThumbHeight = 256
)

type Service struct {
	home         string // Config.ArchivusHome, used to compute relative thumbnail path
	thumbnailDir string // Config.ThumbnailDir

	s3Enabled bool
	store     *store.Store
	s3Manager *s3manager.S3Manager
}

func NewS3Service(s3Manager *s3manager.S3Manager, thumbnailDir string, store *store.Store) *Service {
	return &Service{s3Manager: s3Manager, s3Enabled: true, thumbnailDir: thumbnailDir, store: store}
}

func NewDiskService(home, thumbnailDir string, store *store.Store) *Service {
	return &Service{home: home, thumbnailDir: thumbnailDir, store: store}
}

// GenerateThumbnail generates a JPEG thumbnail for the file at pathKey and
// stores it in the backend-appropriate thumbnail location. Returns the thumbnail
// key (S3) or absolute path (disk). Supported types are images (JPEG, PNG, GIF),
// videos (via ffmpeg) and PDFs (via ghostscript). Returns an error for any other
// file type.
func (s *Service) GenerateThumbnail(ctx context.Context, pathKey string) (string, error) {
	if !s.s3Enabled {
		pathKey = filepath.Join(s.home, pathKey)
	}
	switch strings.ToLower(filepath.Ext(pathKey)) {
	case ".jpg", ".jpeg", ".png", ".gif":
		return s.generateImageThumbnail(ctx, pathKey)
	case ".mp4", ".mov", ".mkv", ".avi", ".webm", ".m4v":
		return s.generateVideoThumbnail(ctx, pathKey)
	case ".pdf":
		return s.generatePDFThumbnail(ctx, pathKey)
	default:
		return "", fmt.Errorf("unsupported file type for thumbnail %q", pathKey)
	}
}

// generateImageThumbnail decodes a supported image, downscales it and writes a
// JPEG thumbnail.
func (s *Service) generateImageThumbnail(ctx context.Context, pathKey string) (string, error) {
	r, err := s.openReader(ctx, pathKey)
	if err != nil {
		return "", err
	}
	defer r.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read image %q: %w", pathKey, err)
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("not a supported image %q: %w", pathKey, err)
	}

	// JPEGs are stored in their sensor orientation and rely on an EXIF
	// orientation tag to be displayed upright. Apply that tag so portrait
	// thumbnails aren't emitted rotated onto their side.
	src = applyEXIFOrientation(src, data)

	thumb := downscale(src, maxThumbWidth, maxThumbHeight)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 80}); err != nil {
		return "", fmt.Errorf("encode thumbnail: %w", err)
	}

	return s.writeThumb(pathKey, buf.Bytes())
}

// applyEXIFOrientation normalises src using the JPEG EXIF orientation tag. It
// returns src unchanged when the image has no orientation tag (or is already
// upright), so non-JPEG formats and untagged images are unaffected.
func applyEXIFOrientation(src image.Image, data []byte) image.Image {
	orientation, err := exifOrientation(data)
	if err != nil || orientation <= 1 {
		return src
	}
	return transformOrientation(src, orientation)
}

// exifOrientation reads the EXIF orientation tag (1-8) from raw image bytes.
func exifOrientation(data []byte) (int, error) {
	x, err := exif.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return 0, err
	}
	return tag.Int(0)
}

// transformOrientation applies an EXIF orientation value (2-8) to src and
// returns the resulting image. The mapping mirrors the EXIF 2.32 spec.
func transformOrientation(src image.Image, orientation int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()

	var outW, outH int
	var mapXY func(x, y int) (int, int)
	switch orientation {
	case 2: // flip horizontal
		outW, outH = w, h
		mapXY = func(x, y int) (int, int) { return w - 1 - x, y }
	case 3: // rotate 180
		outW, outH = w, h
		mapXY = func(x, y int) (int, int) { return w - 1 - x, h - 1 - y }
	case 4: // flip vertical
		outW, outH = w, h
		mapXY = func(x, y int) (int, int) { return x, h - 1 - y }
	case 5: // transpose
		outW, outH = h, w
		mapXY = func(x, y int) (int, int) { return y, x }
	case 6: // rotate 90 CW
		outW, outH = h, w
		mapXY = func(x, y int) (int, int) { return y, h - 1 - x }
	case 7: // transverse
		outW, outH = h, w
		mapXY = func(x, y int) (int, int) { return w - 1 - y, h - 1 - x }
	case 8: // rotate 90 CCW
		outW, outH = h, w
		mapXY = func(x, y int) (int, int) { return w - 1 - y, x }
	default:
		return src
	}

	dst := image.NewNRGBA(image.Rect(0, 0, outW, outH))
	for y := range outH {
		for x := range outW {
			sx, sy := mapXY(x, y)
			dst.Set(x, y, src.At(b.Min.X+sx, b.Min.Y+sy))
		}
	}
	return dst
}

func (s *Service) openReader(ctx context.Context, pathKey string) (io.ReadCloser, error) {
	if s.s3Enabled {
		obj, err := s.s3Manager.Client.GetObject(ctx, s.s3Manager.Client.BucketName, pathKey)
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

// localSourcePath returns a path on the local filesystem for the source file
// along with a cleanup func the caller must always defer. External tools
// (ffmpeg, ghostscript) need a real file on disk: for disk backends this is the
// file itself; for S3 the object is first downloaded to a temp file.
func (s *Service) localSourcePath(ctx context.Context, pathKey string) (string, func(), error) {
	if !s.s3Enabled {
		return pathKey, func() {}, nil
	}
	r, err := s.openReader(ctx, pathKey)
	if err != nil {
		return "", func() {}, err
	}
	defer r.Close()

	tmp, err := os.CreateTemp("", "archivus-src-*"+filepath.Ext(pathKey))
	if err != nil {
		return "", func() {}, fmt.Errorf("create temp file: %w", err)
	}
	cleanup := func() { os.Remove(tmp.Name()) }
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("download s3 object %q: %w", pathKey, err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("write temp file: %w", err)
	}
	return tmp.Name(), cleanup, nil
}

// writeThumb always writes the thumbnail to the local thumbnail directory,
// regardless of whether S3 is enabled. When S3 is enabled the source image is
// read from S3, but thumbnails are always stored on local disk.
func (s *Service) writeThumb(pathKey string, data []byte) (string, error) {
	thumbPath := localThumbnailPath(s.home, s.thumbnailDir, pathKey)
	if err := os.MkdirAll(filepath.Dir(thumbPath), 0755); err != nil {
		return "", fmt.Errorf("create thumbnail dir for %q: %w", thumbPath, err)
	}
	if err := os.WriteFile(thumbPath, data, 0644); err != nil {
		return "", fmt.Errorf("write thumbnail %q: %w", thumbPath, err)
	}
	return thumbPath, nil
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

func (s *Service) MakeThumbnails(ctx context.Context) error {
	fmds, err := s.store.GetFileMetadatasWoThumbnails(ctx, 100) // Adjust limit as needed
	if err != nil {
		return fmt.Errorf("get file metadatas: %w", err)
	}
	if len(fmds) == 0 {
		return nil // No files without thumbnails
	}
	for _, fmd := range fmds {
		thumbnailKey, err := s.GenerateThumbnail(ctx, fmd.PathKey)
		if err != nil {
			logging.CronErrorLogger.Error().Err(err).Str("path", fmd.PathKey).Msg("cron: generate thumbnail")
			continue
		}
		if err := s.store.UpdateFileMetadataThumbnailPath(fmd.ID.String(), thumbnailKey); err != nil {
			logging.CronErrorLogger.Error().Err(err).Str("path", fmd.PathKey).Msg("cron: update thumbnail path")
		}
	}
	return nil
}
