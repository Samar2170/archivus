package thumbnail

// import (
// 	"archivus/pkg/utils"
// 	"errors"
// 	"fmt"
// 	"os"
// 	"os/exec"
// 	"path/filepath"

// 	"github.com/disintegration/imaging"
// )

// func ensureVideoThumbnail(pathKey string) (string, error) {

// 	// Pipe stdout to avoid ffmpeg's image2 muxer failing on paths with spaces
// 	cmd := exec.Command("ffmpeg",
// 		"-y",
// 		"-i", fullSourcePath,
// 		"-ss", "00:00:01",
// 		"-vframes", "1",
// 		"-vf", "scale=200:-2",
// 		"-f", "mjpeg",
// 		"pipe:1",
// 	)
// 	jpegData, err := cmd.Output()
// 	if err != nil {
// 		var exitErr *exec.ExitError
// 		if errors.As(err, &exitErr) {
// 			utils.LogError("ensureVideoThumbnail", "Failed to generate video thumbnail", fmt.Errorf("%w\nstderr: %s", err, exitErr.Stderr))
// 		} else {
// 			utils.LogError("ensureVideoThumbnail", "Failed to generate video thumbnail", err)
// 		}
// 		return "", nil
// 	}

// 	if err := os.WriteFile(fullThumbPath, jpegData, 0644); err != nil {
// 		utils.LogError("ensureVideoThumbnail", "Failed to write video thumbnail", err)
// 		return "", nil
// 	}

// 	return thumbRelPath, nil
// }

// func ensurePDFThumbnail(pathKey string) (string, error) {

// 	if _, err := os.Stat(fullThumbPath); err == nil {
// 		return thumbRelPath, nil
// 	}

// 	if err := prepareThumbnailDir(filepath.Dir(fullThumbPath)); err != nil {
// 		return "", utils.HandleError("ensurePDFThumbnail", "Failed to prepare thumbnail directory", err)
// 	}

// 	// Render first page to a temp JPEG, then resize with imaging
// 	tmpFile := fullThumbPath + ".tmp.jpg"
// 	defer os.Remove(tmpFile)

// 	cmd := exec.Command("gs",
// 		"-dNOPAUSE", "-dBATCH",
// 		"-sDEVICE=jpeg",
// 		"-r72",
// 		"-dFirstPage=1", "-dLastPage=1",
// 		"-sOutputFile="+tmpFile,
// 		fullSourcePath,
// 	)
// 	if out, err := cmd.CombinedOutput(); err != nil {
// 		utils.LogError("ensurePDFThumbnail", "Failed to render PDF", fmt.Errorf("%w\noutput: %s", err, out))
// 		return "", nil
// 	}

// 	img, err := imaging.Open(tmpFile)
// 	if err != nil {
// 		utils.LogError("ensurePDFThumbnail", "Failed to open PDF render", err)
// 		return "", nil
// 	}

// 	thumbnail := imaging.Fit(img, 200, 200, imaging.Lanczos)
// 	if err = imaging.Save(thumbnail, fullThumbPath); err != nil {
// 		utils.LogError("ensurePDFThumbnail", "Failed to save PDF thumbnail", err)
// 		return "", nil
// 	}

// 	return thumbRelPath, nil
// }
