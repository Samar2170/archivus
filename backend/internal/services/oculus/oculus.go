// Package oculus backfills metadata derived from file paths: it fills in the
// missing extension column for all files and flags image files so
// galleries/filters can rely on them.
package oculus

import (
	"archivus/internal/models"
	"archivus/internal/store"
	"fmt"

	"github.com/google/uuid"
)

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
	fmt.Println("Marking extensions for", len(fmds), "files")
	for _, fmd := range fmds {
		ext := models.SplitExtension(fmd.PathKey)
		if ext == "" {
			continue
		}
		extensionsByID[fmd.ID] = ext
		if models.ImageVideoExtensions[ext] {
			imageIDs = append(imageIDs, fmd.ID)
		}
	}
	fmt.Println(extensionsByID)
	if err := s.store.UpdateFileMetadataExtensions(extensionsByID); err != nil {
		return err
	}
	return s.store.MarkFileMetadatasAsImages(imageIDs)
}
