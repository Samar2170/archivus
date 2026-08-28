package store

import (
	"archivus/internal/models"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// FolderSubtree is the directory and file metadata rows that make up one
// folder's contents. Root is the folder itself; Dirs includes Root, Files are
// every file anywhere under it (any nesting depth).
type FolderSubtree struct {
	Root  models.DirectoryMetadata
	Dirs  []models.DirectoryMetadata
	Files []models.FileMetadata
}

// GetDirectoryByDrivePathKey looks up a single live directory row by its exact
// key within a drive. Returns ErrRecordNotFound when no such folder exists.
func (s *Store) GetDirectoryByDrivePathKey(driveID, pathKey string) (models.DirectoryMetadata, error) {
	var dir models.DirectoryMetadata
	result := s.conn().Where("drive_id = ? AND path_key = ?", driveID, pathKey).First(&dir)
	return dir, result.Error
}

// ListFolderSubtreeUnscoped returns every directory and file row belonging to
// the folder rooted at rootPathKey within driveID, regardless of soft-delete
// state: hidden rows still have their original keys, so restore and purge can
// find them. Live rows satisfy !row.DeletedAt.Valid.
//
// Directories match on path_key = rootPathKey or path_key LIKE rootPathKey/%;
// files match on prefix equal to the root key (with and without trailing slash,
// S3 and disk conventions) or any deeper directory's key.
func (s *Store) ListFolderSubtreeUnscoped(driveID, rootPathKey string) (*FolderSubtree, error) {
	subtree := &FolderSubtree{}
	q := s.conn().Unscoped().Where("drive_id = ? AND (path_key = ? OR path_key LIKE ?)", driveID, rootPathKey, rootPathKey+"/%").
		Order("path_key ASC").Find(&subtree.Dirs)
	if q.Error != nil {
		return nil, fmt.Errorf("store: list folder directories under %q: %w", rootPathKey, q.Error)
	}
	if len(subtree.Dirs) == 0 {
		return nil, fmt.Errorf("store: folder %q not found in drive %s", rootPathKey, driveID)
	}
	subtree.Root = subtree.Dirs[0]
	if err := s.conn().Unscoped().Where(
		"drive_id = ? AND (prefix = ? OR prefix = ? OR prefix LIKE ?)",
		driveID, rootPathKey, rootPathKey+"/", rootPathKey+"/%",
	).Find(&subtree.Files).Error; err != nil {
		return nil, fmt.Errorf("store: list folder files under %q: %w", rootPathKey, err)
	}
	return subtree, nil
}

// HideFolderSubtree soft-deletes all live directory and file rows of the folder
// rooted at rootPathKey in one transaction. Rows keep their original keys so a
// recycle bin item alone is enough to put the whole tree back later.
func (s *Store) HideFolderSubtree(driveID, rootPathKey string) error {
	err := s.Transaction(func(tx *Store) error {
		if err := tx.conn().Where("drive_id = ? AND (path_key = ? OR path_key LIKE ?)", driveID, rootPathKey, rootPathKey+"/%").
			Delete(&models.DirectoryMetadata{}).Error; err != nil {
			return fmt.Errorf("store: hide folders under %q: %w", rootPathKey, err)
		}
		if err := tx.conn().Where(
			"drive_id = ? AND (prefix = ? OR prefix = ? OR prefix LIKE ?)",
			driveID, rootPathKey, rootPathKey+"/", rootPathKey+"/%",
		).Delete(&models.FileMetadata{}).Error; err != nil {
			return fmt.Errorf("store: hide files under %q: %w", rootPathKey, err)
		}
		return nil
	})
	return err
}

// UnhideFolderSubtree reactivates the rows that were soft-deleted together with
// a folder recycle item. hiddenSince guards against resurrecting rows from an
// unrelated, older generation of the same path: only rows hidden at or after
// the item was created come back.
func (s *Store) UnhideFolderSubtree(driveID, rootPathKey string, hiddenSince time.Time) error {
	return s.Transaction(func(tx *Store) error {
		if err := tx.conn().Unscoped().Model(&models.DirectoryMetadata{}).
			Where("drive_id = ? AND (path_key = ? OR path_key LIKE ?) AND deleted_at >= ?", driveID, rootPathKey, rootPathKey+"/%", hiddenSince).
			Update("deleted_at", gorm.Expr("NULL")).Error; err != nil {
			return fmt.Errorf("store: unhide folders under %q: %w", rootPathKey, err)
		}
		if err := tx.conn().Unscoped().Model(&models.FileMetadata{}).
			Where("drive_id = ? AND (prefix = ? OR prefix = ? OR prefix LIKE ?) AND deleted_at >= ?", driveID, rootPathKey, rootPathKey+"/", rootPathKey+"/%", hiddenSince).
			Update("deleted_at", gorm.Expr("NULL")).Error; err != nil {
			return fmt.Errorf("store: unhide files under %q: %w", rootPathKey, err)
		}
		return nil
	})
}

// HardDeleteHiddenFolderSubtree permanently removes the metadata rows hidden
// along with a folder recycle item — the purge job calls this once the recycled
// bytes are gone. Active rows can never match because they have no deleted_at.
func (s *Store) HardDeleteHiddenFolderSubtree(driveID, rootPathKey string, hiddenSince time.Time) error {
	return s.Transaction(func(tx *Store) error {
		if err := tx.conn().Unscoped().
			Where("drive_id = ? AND (path_key = ? OR path_key LIKE ?) AND deleted_at >= ?", driveID, rootPathKey, rootPathKey+"/%", hiddenSince).
			Delete(&models.DirectoryMetadata{}).Error; err != nil {
			return fmt.Errorf("store: hard delete hidden folders under %q: %w", rootPathKey, err)
		}
		if err := tx.conn().Unscoped().
			Where("drive_id = ? AND (prefix = ? OR prefix = ? OR prefix LIKE ?) AND deleted_at >= ?", driveID, rootPathKey, rootPathKey+"/", rootPathKey+"/%", hiddenSince).
			Delete(&models.FileMetadata{}).Error; err != nil {
			return fmt.Errorf("store: hard delete hidden files under %q: %w", rootPathKey, err)
		}
		return nil
	})
}

// ListHiddenFolderThumbnails returns the thumbnail paths of the files hidden
// along with a folder recycle item, so the purge job can clean them up too.
func (s *Store) ListHiddenFolderThumbnails(driveID, rootPathKey string, hiddenSince time.Time) ([]string, error) {
	var thumbs []string
	if err := s.conn().Unscoped().Model(&models.FileMetadata{}).
		Where("drive_id = ? AND (prefix = ? OR prefix = ? OR prefix LIKE ?) AND deleted_at >= ? AND thumbnail_path <> ''",
			driveID, rootPathKey, rootPathKey+"/", rootPathKey+"/%", hiddenSince).
		Pluck("thumbnail_path", &thumbs).Error; err != nil {
		return nil, fmt.Errorf("store: list hidden folder thumbnails under %q: %w", rootPathKey, err)
	}
	return thumbs, nil
}
