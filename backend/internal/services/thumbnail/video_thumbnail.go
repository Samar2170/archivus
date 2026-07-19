package thumbnail

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"os/exec"
)

// generateVideoThumbnail extracts a single frame ~1s into the video with ffmpeg,
// scales it to the thumbnail width and writes it as JPEG.
func (s *Service) generateVideoThumbnail(ctx context.Context, pathKey string) (string, error) {
	src, cleanup, err := s.localSourcePath(ctx, pathKey)
	if err != nil {
		return "", err
	}
	defer cleanup()

	// Pipe stdout to avoid ffmpeg's image2 muxer failing on paths with spaces.
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y",
		"-i", src,
		"-ss", "00:00:01",
		"-vframes", "1",
		"-vf", fmt.Sprintf("scale=%d:-2", maxThumbWidth),
		"-f", "mjpeg",
		"pipe:1",
	)
	jpegData, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("ffmpeg thumbnail for %q: %w\nstderr: %s", pathKey, err, exitErr.Stderr)
		}
		return "", fmt.Errorf("ffmpeg thumbnail for %q: %w", pathKey, err)
	}

	return s.writeThumb(pathKey, jpegData)
}

// generatePDFThumbnail renders the first page of a PDF to JPEG with ghostscript,
// then downscales it to the thumbnail bounds and writes it.
func (s *Service) generatePDFThumbnail(ctx context.Context, pathKey string) (string, error) {
	src, cleanup, err := s.localSourcePath(ctx, pathKey)
	if err != nil {
		return "", err
	}
	defer cleanup()

	tmpOut, err := os.CreateTemp("", "archivus-pdf-*.jpg")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpOut.Close()
	defer os.Remove(tmpOut.Name())

	cmd := exec.CommandContext(ctx, "gs",
		"-dNOPAUSE", "-dBATCH", "-dQUIET",
		"-sDEVICE=jpeg",
		"-r72",
		"-dFirstPage=1", "-dLastPage=1",
		"-sOutputFile="+tmpOut.Name(),
		src,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("gs render pdf %q: %w\noutput: %s", pathKey, err, out)
	}

	f, err := os.Open(tmpOut.Name())
	if err != nil {
		return "", fmt.Errorf("open pdf render %q: %w", pathKey, err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return "", fmt.Errorf("decode pdf render %q: %w", pathKey, err)
	}

	thumb := downscale(img, maxThumbWidth, maxThumbHeight)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 80}); err != nil {
		return "", fmt.Errorf("encode pdf thumbnail: %w", err)
	}

	return s.writeThumb(pathKey, buf.Bytes())
}
