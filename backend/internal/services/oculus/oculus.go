// Package oculus backfills metadata derived from file paths: it fills in the
// missing extension column for all files and flags image files so
// galleries/filters can rely on them.
package oculus

import (
	"path/filepath"
	"strings"

	"archivus/internal/store"

	"github.com/google/uuid"
)

// imageExtensions are the (dot-less, lowercase) extensions treated as images.
var imageVideoExtensions = map[string]bool{
	"jpg":  true,
	"jpeg": true,
	"png":  true,
	"gif":  true,
	"webp": true,
	"bmp":  true,
	"tiff": true,
	"svg":  true,
	"heic": true,

	"mp4":  true,
	"mov":  true,
	"mkv":  true,
	"avi":  true,
	"webm": true,
	"m4v":  true,
}

type Service struct {
	store *store.Store
}

func NewService(store *store.Store) *Service {
	return &Service{store: store}
}

// MarkImages processes one batch of files that have no extension recorded:
// it derives each extension from the file's path key, writes all extensions
// back in a single query, then flags the image files in a second single query.
// Files whose path key has no extension are left untouched.
func (s *Service) MarkImages() error {
	fmds, err := s.store.GetFileMetadatasWoExtension(100)
	if err != nil {
		return err
	}
	if len(fmds) == 0 {
		return nil
	}

	extensionsByID := make(map[uuid.UUID]string, len(fmds))
	var imageIDs []uuid.UUID
	for _, fmd := range fmds {
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fmd.PathKey), "."))
		if ext == "" {
			continue
		}
		extensionsByID[fmd.ID] = ext
		if imageVideoExtensions[ext] {
			imageIDs = append(imageIDs, fmd.ID)
		}
	}

	if err := s.store.UpdateFileMetadataExtensions(extensionsByID); err != nil {
		return err
	}
	return s.store.MarkFileMetadatasAsImages(imageIDs)
}
