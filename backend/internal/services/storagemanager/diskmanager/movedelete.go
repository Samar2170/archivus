package diskmanager

import (
	archivus_constants "archivus/internal/constants"
	"archivus/internal/models"
	"archivus/pkg/logging"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// MoveFile relocates a file into dstFolderRelPath (relative to the drive root),
// keeping its name. The bytes are renamed on disk and the metadata row's
// key/prefix are updated to match; a DB failure reverts the on-disk move.
func (dm *DiskManager) MoveFile(fileId, dstFolderRelPath, driveId, userId string) error {
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
	md, err := dm.Store.GetFileMetadataByID(fileId)
	if err != nil {
		return fmt.Errorf("diskmanager: get file metadata by id %q: %w", fileId, err)
	}
	if md.DriveID != drive.ID {
		return errors.New("file does not belong to this drive")
	}

	dstPrefix := filepath.Join(dm.Home, drive.Slug, dstFolderRelPath)
	dstPathKey := filepath.Join(dstPrefix, md.Name)
	if dstPathKey == md.PathKey {
		return nil // already there
	}
	if _, err := os.Stat(dstPathKey); err == nil {
		return errors.New("a file with the same name already exists at the destination")
	}
	if err := os.MkdirAll(dstPrefix, 0o755); err != nil {
		return fmt.Errorf("diskmanager: create destination dir %q: %w", dstPrefix, err)
	}
	if err := os.Rename(md.PathKey, dstPathKey); err != nil {
		return fmt.Errorf("diskmanager: move file %q to %q: %w", md.PathKey, dstPathKey, err)
	}
	if err := dm.Store.UpdateFileMetadataLocation(md.ID.String(), dstPathKey, dstPrefix); err != nil {
		if rerr := os.Rename(dstPathKey, md.PathKey); rerr != nil {
			log.Warn().Err(rerr).Msg("diskmanager: failed to revert file move after db error")
		}
		return fmt.Errorf("diskmanager: update file metadata after move: %w", err)
	}
	return nil
}

// DeleteFileV2 moves a file into the recycle bin: its bytes are relocated under
// the recycle bin directory, a RecycleBinItem row is recorded (so it can be
// restored or purged), and the original file metadata row is removed so the
// file no longer appears in listings.
func (dm *DiskManager) DeleteFileV2(fileId, driveId, userId string) error {
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
	md, err := dm.Store.GetFileMetadataByID(fileId)
	if err != nil {
		return fmt.Errorf("diskmanager: get file metadata by id %q: %w", fileId, err)
	}
	if md.DriveID != drive.ID {
		return errors.New("file does not belong to this drive")
	}

	recycleDir := filepath.Join(dm.Home, archivus_constants.RecycleBinDirName, uuid.New().String())
	recyclePathKey := filepath.Join(recycleDir, md.Name)
	if err := os.MkdirAll(recycleDir, 0o755); err != nil {
		return fmt.Errorf("diskmanager: create recycle bin dir %q: %w", recycleDir, err)
	}
	if err := os.Rename(md.PathKey, recyclePathKey); err != nil {
		return fmt.Errorf("diskmanager: move file %q to recycle bin: %w", md.PathKey, err)
	}

	expiresAt := time.Now().AddDate(0, 0, archivus_constants.RecycleBinRetentionDays)
	if _, err := dm.Store.CreateRecycleBinItem(md.Name, md.PathKey, md.Prefix, recyclePathKey, md.ContentType, md.ThumbnailPath, driveId, userId, md.SizeInMb, expiresAt, false); err != nil {
		if rerr := os.Rename(recyclePathKey, md.PathKey); rerr != nil {
			log.Warn().Err(rerr).Msg("diskmanager: failed to restore file after recycle bin db error")
		}
		return fmt.Errorf("diskmanager: record recycle bin item for %q: %w", md.PathKey, err)
	}
	if err := dm.Store.DeleteFileMetadataByID(md.ID.String()); err != nil {
		return fmt.Errorf("diskmanager: delete file metadata for %q: %w", md.PathKey, err)
	}
	return nil
}

// RestoreFile moves a recycle bin item back to its original location and
// recreates its file metadata row.
func (dm *DiskManager) RestoreFile(recycleBinId, driveId, userId string) error {
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
	item, err := dm.Store.GetRecycleBinItemByID(recycleBinId)
	if err != nil {
		return fmt.Errorf("diskmanager: get recycle bin item %q: %w", recycleBinId, err)
	}
	if item.DriveID != drive.ID {
		return errors.New("recycle bin item does not belong to this drive")
	}
	// Folder items move their whole tree back and revive the metadata rows
	// hidden when the folder was deleted; files follow the classic path.
	if item.IsDir {
		return dm.restoreFolder(item)
	}
	if _, err := os.Stat(item.OriginalPathKey); err == nil {
		return errors.New("a file already exists at the original location")
	}
	if err := os.MkdirAll(item.OriginalPrefix, 0o755); err != nil {
		return fmt.Errorf("diskmanager: recreate dir %q: %w", item.OriginalPrefix, err)
	}
	if err := os.Rename(item.RecyclePathKey, item.OriginalPathKey); err != nil {
		return fmt.Errorf("diskmanager: restore file to %q: %w", item.OriginalPathKey, err)
	}
	if _, err := dm.Store.CreateFileMetadataV2(item.Name, item.OriginalPathKey, item.OriginalPrefix, item.ContentType, driveId, userId, item.SizeInMb); err != nil {
		if rerr := os.Rename(item.OriginalPathKey, item.RecyclePathKey); rerr != nil {
			log.Warn().Err(rerr).Msg("diskmanager: failed to re-recycle file after restore db error")
		}
		return fmt.Errorf("diskmanager: recreate file metadata on restore: %w", err)
	}
	if err := dm.Store.DeleteRecycleBinItemByID(item.ID.String()); err != nil {
		return fmt.Errorf("diskmanager: delete recycle bin item after restore: %w", err)
	}
	if err := os.Remove(filepath.Dir(item.RecyclePathKey)); err != nil && !os.IsNotExist(err) {
		log.Warn().Err(err).Msg("diskmanager: failed to remove empty recycle bin dir")
	}
	return nil
}

// PurgeExpiredRecycleBin permanently removes recycle bin items whose retention
// window has elapsed, deleting the stored bytes, any thumbnail, and the row.
func (dm *DiskManager) PurgeExpiredRecycleBin(ctx context.Context) error {
	items, err := dm.Store.GetExpiredRecycleBinItems(time.Now())
	if err != nil {
		return fmt.Errorf("diskmanager: list expired recycle bin items: %w", err)
	}
	for _, it := range items {
		if err := ctx.Err(); err != nil {
			return err
		}
		var purgeErr error
		if it.IsDir {
			purgeErr = dm.purgeFolderItem(it)
		} else {
			purgeErr = dm.purgeFileItem(it)
		}
		if purgeErr != nil {
			logging.CronErrorLogger.Error().Err(purgeErr).Str("path", it.RecyclePathKey).Msg("cron: purge: failed to remove recycled item")
			continue
		}
		if err := dm.Store.DeleteRecycleBinItemByID(it.ID.String()); err != nil {
			logging.CronErrorLogger.Error().Err(err).Str("id", it.ID.String()).Msg("cron: purge: failed to delete recycle bin row")
		}
	}
	log.Info().Int("count", len(items)).Msg("diskmanager: purged expired recycle bin items")
	return nil
}

// purgeFileItem permanently removes a recycled file: its bytes and any
// thumbnail.
func (dm *DiskManager) purgeFileItem(it models.RecycleBinItem) error {
	if err := os.RemoveAll(filepath.Dir(it.RecyclePathKey)); err != nil {
		return fmt.Errorf("remove recycled file %q: %w", it.RecyclePathKey, err)
	}
	if it.ThumbnailPath != "" {
		if err := os.Remove(it.ThumbnailPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove thumbnail %q: %w", it.ThumbnailPath, err)
		}
	}
	return nil
}
