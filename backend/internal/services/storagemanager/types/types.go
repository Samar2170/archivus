package types

import (
	archivus_constants "archivus/internal/constants"
	"path/filepath"
	"strings"
	"time"
)

// RecycleEntry is a single file held in the recycle bin, as shown to the user.
type RecycleEntry struct {
	ID           string
	Name         string
	Size         float64
	ContentType  string
	OriginalPath string
	DeletedAt    time.Time
	ExpiresAt    time.Time
}

type DirEntry struct {
	ID        string
	Name      string
	IsDir     bool
	Extension string
	SignedUrl string
	Size      float64
	Path      string
	Thumbnail string

	NavigationPath string
}

// ThumbnailURL maps a stored (absolute) thumbnail path under thumbnailDir to the
// URL path served by the static thumbnail file server. Returns "" when there is
// no thumbnail or the path escapes thumbnailDir.
func ThumbnailURL(thumbnailPath, thumbnailDir string) string {
	if thumbnailPath == "" {
		return ""
	}
	rel, err := filepath.Rel(thumbnailDir, thumbnailPath)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	return archivus_constants.ThumbnailRoutePrefix + filepath.ToSlash(rel)
}
