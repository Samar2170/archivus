package s3manager

import (
	archivus_constants "archivus/internal/constants"
	"archivus/pkg/logging"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// MoveFile relocates a file into dstFolderRelPath (relative to the drive root),
// keeping its name. S3 has no rename, so the object is copied to the new key and
// the old key deleted, then the metadata row is updated to match.
func (s *S3Manager) MoveFile(fileId, dstFolderRelPath, driveId, userId string) error {
	hasAccess, err := s.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("user does not have write access to this drive")
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	md, err := s.Store.GetFileMetadataByID(fileId)
	if err != nil {
		return fmt.Errorf("s3manager: get file metadata %q: %w", fileId, err)
	}
	if md.DriveID != drive.ID {
		return errors.New("file does not belong to this drive")
	}

	trimmed := strings.Trim(dstFolderRelPath, "/")
	var dstPathKey, dstPrefix string
	if trimmed == "" {
		dstPrefix = drive.Slug + "/"
		dstPathKey = dstPrefix + md.Name
	} else {
		dstPrefix = drive.Slug + "/" + trimmed + "/"
		dstPathKey = dstPrefix + md.Name
	}
	if dstPathKey == md.PathKey {
		return nil // already there
	}
	ctx := context.Background()
	if err := s.Client.CopyObject(ctx, s.Client.BucketName, md.PathKey, dstPathKey); err != nil {
		return fmt.Errorf("s3manager: copy %q to %q: %w", md.PathKey, dstPathKey, err)
	}
	if err := s.Client.DeleteObject(ctx, s.Client.BucketName, md.PathKey); err != nil {
		_ = s.Client.DeleteObject(ctx, s.Client.BucketName, dstPathKey)
		return fmt.Errorf("s3manager: delete source %q after move: %w", md.PathKey, err)
	}
	if err := s.Store.UpdateFileMetadataLocation(md.ID.String(), dstPathKey, dstPrefix); err != nil {
		return fmt.Errorf("s3manager: update file metadata after move: %w", err)
	}
	return nil
}

// DeleteFileV2 moves a file into the recycle bin: the object is copied under the
// recycle bin key prefix and the original deleted, a RecycleBinItem row is
// recorded, and the original file metadata row is removed.
func (s *S3Manager) DeleteFileV2(fileId, driveId, userId string) error {
	hasAccess, err := s.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("user does not have write access to this drive")
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	md, err := s.Store.GetFileMetadataByID(fileId)
	if err != nil {
		return fmt.Errorf("s3manager: get file metadata %q: %w", fileId, err)
	}
	if md.DriveID != drive.ID {
		return errors.New("file does not belong to this drive")
	}

	recycleKey := archivus_constants.RecycleBinDirName + "/" + uuid.New().String() + "/" + md.Name
	ctx := context.Background()
	if err := s.Client.CopyObject(ctx, s.Client.BucketName, md.PathKey, recycleKey); err != nil {
		return fmt.Errorf("s3manager: copy %q to recycle bin: %w", md.PathKey, err)
	}
	if err := s.Client.DeleteObject(ctx, s.Client.BucketName, md.PathKey); err != nil {
		_ = s.Client.DeleteObject(ctx, s.Client.BucketName, recycleKey)
		return fmt.Errorf("s3manager: delete original %q: %w", md.PathKey, err)
	}

	expiresAt := time.Now().AddDate(0, 0, archivus_constants.RecycleBinRetentionDays)
	if _, err := s.Store.CreateRecycleBinItem(md.Name, md.PathKey, md.Prefix, recycleKey, md.ContentType, md.ThumbnailPath, driveId, userId, md.SizeInMb, expiresAt); err != nil {
		// Try to restore the object so the delete is not silently lost.
		if rerr := s.Client.CopyObject(ctx, s.Client.BucketName, recycleKey, md.PathKey); rerr == nil {
			_ = s.Client.DeleteObject(ctx, s.Client.BucketName, recycleKey)
		} else {
			log.Warn().Err(rerr).Msg("s3manager: failed to restore object after recycle bin db error")
		}
		return fmt.Errorf("s3manager: record recycle bin item for %q: %w", md.PathKey, err)
	}
	if err := s.Store.DeleteFileMetadataByID(md.ID.String()); err != nil {
		return fmt.Errorf("s3manager: delete file metadata for %q: %w", md.PathKey, err)
	}
	return nil
}

// RestoreFile moves a recycle bin item back to its original key and recreates
// its file metadata row.
func (s *S3Manager) RestoreFile(recycleBinId, driveId, userId string) error {
	hasAccess, err := s.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("user does not have write access to this drive")
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	item, err := s.Store.GetRecycleBinItemByID(recycleBinId)
	if err != nil {
		return fmt.Errorf("s3manager: get recycle bin item %q: %w", recycleBinId, err)
	}
	if item.DriveID != drive.ID {
		return errors.New("recycle bin item does not belong to this drive")
	}
	ctx := context.Background()
	if err := s.Client.CopyObject(ctx, s.Client.BucketName, item.RecyclePathKey, item.OriginalPathKey); err != nil {
		return fmt.Errorf("s3manager: restore object to %q: %w", item.OriginalPathKey, err)
	}
	if err := s.Client.DeleteObject(ctx, s.Client.BucketName, item.RecyclePathKey); err != nil {
		log.Warn().Err(err).Msg("s3manager: failed to remove recycled object after restore")
	}
	if _, err := s.Store.CreateFileMetadataV2(item.Name, item.OriginalPathKey, item.OriginalPrefix, item.ContentType, driveId, userId, item.SizeInMb); err != nil {
		return fmt.Errorf("s3manager: recreate file metadata on restore: %w", err)
	}
	if err := s.Store.DeleteRecycleBinItemByID(item.ID.String()); err != nil {
		return fmt.Errorf("s3manager: delete recycle bin item after restore: %w", err)
	}
	return nil
}

// PurgeExpiredRecycleBin permanently removes recycle bin items whose retention
// window has elapsed, deleting the stored object, any local thumbnail, and the
// row.
func (s *S3Manager) PurgeExpiredRecycleBin(ctx context.Context) error {
	items, err := s.Store.GetExpiredRecycleBinItems(time.Now())
	if err != nil {
		return fmt.Errorf("s3manager: list expired recycle bin items: %w", err)
	}
	for _, it := range items {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.Client.DeleteObject(ctx, s.Client.BucketName, it.RecyclePathKey); err != nil {
			logging.CronErrorLogger.Error().Err(err).Str("key", it.RecyclePathKey).Msg("cron: purge: failed to delete recycled object")
			continue
		}
		if it.ThumbnailPath != "" {
			if err := os.Remove(it.ThumbnailPath); err != nil && !os.IsNotExist(err) {
				logging.CronErrorLogger.Error().Err(err).Str("path", it.ThumbnailPath).Msg("cron: purge: failed to remove thumbnail")
			}
		}
		if err := s.Store.DeleteRecycleBinItemByID(it.ID.String()); err != nil {
			logging.CronErrorLogger.Error().Err(err).Str("id", it.ID.String()).Msg("cron: purge: failed to delete recycle bin row")
		}
	}
	log.Info().Int("count", len(items)).Msg("s3manager: purged expired recycle bin items")
	return nil
}
