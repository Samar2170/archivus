package diskmanager

import (
	archivus_constants "archivus/internal/constants"
	"archivus/internal/models"
	"archivus/internal/services/storagemanager/base"
	"archivus/internal/store"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type DiskManager struct {
	base.BaseManager
	Home      string
	UsersHome string
}

func GetDiskManager(s *store.Store, home string) *DiskManager {
	return &DiskManager{
		BaseManager: base.BaseManager{Store: s},
		Home:        home,
	}
}

var ErrMasterUserPersonalDrive = errors.New("master users cannot have a personal drive")

// CreateDriveDir creates the filesystem directory for a shared drive.
func (dm *DiskManager) CreateDriveDir(slug string) (string, error) {
	absPath := filepath.Join(dm.Home, slug)
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return "", fmt.Errorf("diskmanager: create drive dir %q: %w", absPath, err)
	}
	return dm.Home, nil
}

func (dm *DiskManager) DeleteDriveDir(slug string) error {
	absPath := filepath.Join(dm.Home, slug)
	if err := os.RemoveAll(absPath); err != nil {
		return fmt.Errorf("diskmanager: delete drive dir %q: %w", absPath, err)
	}
	return nil
}

// CreateUserDriveDir creates a personal drive directory for a non-master user.
// func (dm *DiskManager) CreateUserDriveDir(user *models.User) error {
// 	if user.IsMaster {
// 		return ErrMasterUserPersonalDrive
// 	}
// 	if dm.S3Enabled {
// 		return nil
// 	}
// 	path := filepath.Join(dm.UsersHome, user.Username)
// 	if err := os.MkdirAll(path, 0755); err != nil {
// 		return fmt.Errorf("diskmanager: create user drive dir %q: %w", path, err)
// 	}
// 	return nil
// }

func (dm *DiskManager) CreateDir(subFolder, driveId, userId string) error {
	if subFolder == "" {
		return errors.New("subFolder cannot be empty")
	}
	hasAccess, err := dm.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("user does not have write access to this drive")
	}

	drive, err := dm.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("diskmanager: get drive by id %q: %w", driveId, err)
	}

	relPath := filepath.Join(drive.Slug, subFolder)
	dirPath := filepath.Join(dm.Home, relPath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("diskmanager: create dir %q: %w", dirPath, err)
	}
	name := filepath.Base(subFolder)

	_, err = dm.Store.CreateDirectoryMetadata(name, dirPath, relPath, drive.ID.String())
	if err != nil {
		if cleanupErr := os.RemoveAll(dirPath); cleanupErr != nil {
			fmt.Printf("warning: failed to clean up directory after db error: %v\n", cleanupErr)
		}
		return fmt.Errorf("diskmanager: create directory metadata for dir %q: %w", dirPath, err)
	}
	return nil
}

func (dm *DiskManager) CreateDirV2(subFolder, driveId, userId string) error {
	if subFolder == "" {
		return errors.New("subFolder cannot be empty")
	}
	hasAccess, err := dm.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("user does not have write access to this drive")
	}
	drive, err := dm.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("diskmanager: get drive by id %q: %w", driveId, err)
	}
	pathKey := filepath.Join(dm.Home, drive.Slug, subFolder)
	prefix := filepath.Dir(pathKey)
	// Creating a folder that already exists is a no-op rather than an error:
	// clients (the sync client in particular) make a destination exist without
	// checking first, and nothing in the schema would stop a second insert from
	// producing a duplicate row that then shows up twice in listings. Checked
	// before MkdirAll so an existing directory is never a candidate for the
	// cleanup below.
	exists, err := dm.Store.DirectoryExistsByDrivePathKey(drive.ID.String(), pathKey)
	if err != nil {
		return fmt.Errorf("diskmanager: look up directory %q: %w", pathKey, err)
	}
	// Nested paths must have every ancestor recorded, not just the leaf, or the
	// deeper folders are invisible from their parent listings. Mirrors what
	// EnsureDirectoryMetadata does for S3; missing ancestors are no-op-safe to
	// create because sync clients arrive out of order.
	trimmed := strings.Trim(subFolder, "/")
	baseAbs := filepath.Join(dm.Home, drive.Slug)
	relSoFar := ""
	for _, seg := range strings.Split(trimmed, "/") {
		if seg == "" {
			continue
		}
		relSoFar = filepath.Join(relSoFar, seg)
		ancestorPathKey := filepath.Join(baseAbs, relSoFar)
		ancestorExists, err := dm.Store.DirectoryExistsByDrivePathKey(drive.ID.String(), ancestorPathKey)
		if err != nil {
			return fmt.Errorf("diskmanager: look up directory %q: %w", ancestorPathKey, err)
		}
		if !ancestorExists {
			if _, err := dm.Store.CreateDirectoryMetadataV2(filepath.Base(ancestorPathKey), ancestorPathKey, filepath.Dir(ancestorPathKey), drive.ID.String()); err != nil {
				return fmt.Errorf("diskmanager: create ancestor directory metadata %q: %w", ancestorPathKey, err)
			}
		}
	}
	if exists {
		return nil
	}
	if err := os.MkdirAll(pathKey, 0755); err != nil {
		return fmt.Errorf("diskmanager: create dir %q: %w", pathKey, err)
	}
	name := filepath.Base(subFolder)
	_, err = dm.Store.CreateDirectoryMetadataV2(name, pathKey, prefix, drive.ID.String())
	if err != nil {
		if cleanupErr := os.RemoveAll(pathKey); cleanupErr != nil {
			fmt.Printf("warning: failed to clean up directory after db error: %v\n", cleanupErr)
		}
		return fmt.Errorf("diskmanager: create directory metadata for dir %q: %w", pathKey, err)
	}
	return nil
}

// DirExists reports whether relPath is an existing folder in the drive. On local
// disk the filesystem is the source of truth, so it is what gets stat'd.
func (dm *DiskManager) DirExists(relPath, driveId, userId string) (bool, error) {
	hasAccess, err := dm.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return false, err
	}
	if !hasAccess {
		return false, errors.New("user does not have write access to this drive")
	}
	drive, err := dm.Store.GetDriveByID(driveId)
	if err != nil {
		return false, fmt.Errorf("diskmanager: get drive by id %q: %w", driveId, err)
	}
	dirPath := filepath.Join(dm.Home, drive.Slug, relPath)
	info, err := os.Stat(dirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("diskmanager: stat dir %q: %w", dirPath, err)
	}
	return info.IsDir(), nil
}

// DeleteDir moves relPath's folder — and everything inside it — into the
// recycle bin, where it stays for the retention window before being permanently
// purged (mirroring DeleteFileV2 for folders). The bytes are renamed under the
// recycle bin directory as one unit so inner paths never change; the folder's
// metadata rows are soft-deleted in place, keeping their original keys so a
// restore can put them back without rewriting anything.
func (dm *DiskManager) DeleteDir(relPath, driveId, userId string) error {
	hasAccess, err := dm.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("user does not have write access to this drive")
	}

	drive, err := dm.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("diskmanager: get drive by id %q: %w", driveId, err)
	}
	trimmed := strings.Trim(relPath, "/")
	if trimmed == "" {
		return errors.New("cannot delete the drive root")
	}
	pathKey := filepath.Join(dm.Home, drive.Slug, trimmed)
	rootRow, err := dm.Store.GetDirectoryByDrivePathKey(drive.ID.String(), pathKey)
	if err != nil {
		return fmt.Errorf("diskmanager: get directory metadata %q: %w", pathKey, err)
	}
	subtree, err := dm.Store.ListFolderSubtreeUnscoped(drive.ID.String(), pathKey)
	if err != nil {
		return fmt.Errorf("diskmanager: list folder subtree %q: %w", pathKey, err)
	}
	var sizeInMb float64
	for _, f := range subtree.Files {
		if !f.DeletedAt.Valid {
			sizeInMb += f.SizeInMb
		}
	}

	recycleDir := filepath.Join(dm.Home, archivus_constants.RecycleBinDirName, uuid.New().String())
	recyclePathKey := filepath.Join(recycleDir, filepath.Base(trimmed))
	if err := os.MkdirAll(recycleDir, 0o755); err != nil {
		return fmt.Errorf("diskmanager: create recycle bin dir %q: %w", recycleDir, err)
	}
	if err := os.Rename(pathKey, recyclePathKey); err != nil {
		return fmt.Errorf("diskmanager: move folder %q to recycle bin: %w", pathKey, err)
	}

	expiresAt := time.Now().AddDate(0, 0, archivus_constants.RecycleBinRetentionDays)
	err = dm.Store.Transaction(func(tx *store.Store) error {
		if _, err := tx.CreateRecycleBinItem(rootRow.Name, rootRow.PathKey, rootRow.Prefix, recyclePathKey, "", "", driveId, userId, sizeInMb, expiresAt, true); err != nil {
			return err
		}
		return tx.HideFolderSubtree(driveId, pathKey)
	})
	if err != nil {
		if rerr := os.Rename(recyclePathKey, pathKey); rerr != nil {
			log.Warn().Err(rerr).Msg("diskmanager: failed to restore folder after recycle bin db error")
		}
		return fmt.Errorf("diskmanager: record recycle bin item for folder %q: %w", pathKey, err)
	}
	return nil
}

// restoreFolder moves a recycled folder tree back to its original location and
// reactivates its soft-deleted metadata rows.
func (dm *DiskManager) restoreFolder(item models.RecycleBinItem) error {
	dstPathKey := item.OriginalPathKey // absolute path on disk
	if _, err := os.Stat(dstPathKey); err == nil {
		return errors.New("a folder already exists at the original location")
	}
	if err := os.MkdirAll(filepath.Dir(dstPathKey), 0o755); err != nil {
		return fmt.Errorf("diskmanager: recreate parent dir %q: %w", filepath.Dir(dstPathKey), err)
	}
	if err := os.Rename(item.RecyclePathKey, dstPathKey); err != nil {
		return fmt.Errorf("diskmanager: restore folder to %q: %w", dstPathKey, err)
	}
	if err := dm.Store.UnhideFolderSubtree(item.DriveID.String(), dstPathKey, item.CreatedAt); err != nil {
		if rerr := os.Rename(dstPathKey, item.RecyclePathKey); rerr != nil {
			log.Warn().Err(rerr).Msg("diskmanager: failed to re-recycle folder after restore db error")
		}
		return fmt.Errorf("diskmanager: restore folder metadata for %q: %w", dstPathKey, err)
	}
	if err := dm.Store.DeleteRecycleBinItemByID(item.ID.String()); err != nil {
		return fmt.Errorf("diskmanager: delete recycle bin item after restore: %w", err)
	}
	dm.cleanupRecycleDir(item.RecyclePathKey)
	return nil
}

// purgeFolderItem permanently removes a recycled folder: the relocated bytes,
// every thumbnail of the files that lived inside it, and their hidden metadata
// rows. It returns any failure so callers decide how to surface it — the cron
// purge logs and moves on, the immediate-purge API reports it to the user.
func (dm *DiskManager) purgeFolderItem(it models.RecycleBinItem) error {
	thumbs, err := dm.Store.ListHiddenFolderThumbnails(it.DriveID.String(), it.OriginalPathKey, it.CreatedAt)
	if err != nil {
		return fmt.Errorf("list hidden folder thumbnails for %q: %w", it.OriginalPathKey, err)
	}
	for _, thumb := range thumbs {
		if err := os.Remove(thumb); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove thumbnail %q: %w", thumb, err)
		}
	}
	if err := os.RemoveAll(filepath.Dir(it.RecyclePathKey)); err != nil {
		return fmt.Errorf("remove recycled folder %q: %w", it.RecyclePathKey, err)
	}
	if err := dm.Store.HardDeleteHiddenFolderSubtree(it.DriveID.String(), it.OriginalPathKey, it.CreatedAt); err != nil {
		return fmt.Errorf("delete hidden folder rows under %q: %w", it.OriginalPathKey, err)
	}
	return nil
}

// cleanupRecycleDir removes the per-item uuid directory left empty after a file
// or folder was restored out of the recycle bin.
func (dm *DiskManager) cleanupRecycleDir(recyclePathKey string) {
	if err := os.Remove(filepath.Dir(recyclePathKey)); err != nil && !os.IsNotExist(err) {
		log.Warn().Err(err).Msg("diskmanager: failed to remove empty recycle bin dir")
	}
}
