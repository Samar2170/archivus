package s3manager

import (
	archivus_constants "archivus/internal/constants"
	"archivus/internal/models"
	"archivus/internal/services/storagemanager/base"
	"archivus/internal/store"
	"archivus/pkg/logging"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type S3Manager struct {
	base.BaseManager
	Client *Client
}

func GetS3Manager(s *store.Store, accountID, accessKey, secretKey, bucketName string) (*S3Manager, error) {
	client, err := New(accountID, accessKey, secretKey, bucketName)
	if err != nil {
		return nil, err
	}
	return &S3Manager{BaseManager: base.BaseManager{Store: s}, Client: client}, nil
}

func (s *S3Manager) CreateDriveDir(driveName string) (string, error) {
	return driveName, nil
}

// DeleteDriveDir deletes a drive bucket by name. Only useful as error-path cleanup
// immediately after CreateDriveDir, since the slug is re-derived and won't match
// an existing bucket created in a previous call.
func (s *S3Manager) DeleteDriveDir(driveName string) error {
	return nil
}

func (s *S3Manager) CreateDir(subFolder, driveId, userId string) error {
	if subFolder == "" {
		return errors.New("subFolder cannot be empty")
	}
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
	// trailing slash is the S3 convention for virtual directories; key is namespaced by drive slug
	key := drive.Slug + "/" + strings.Trim(subFolder, "/") + "/"
	if err := s.Client.CreateDirectory(context.Background(), s.Client.BucketName, key); err != nil {
		return fmt.Errorf("s3manager: create directory %q in bucket %q: %w", key, s.Client.BucketName, err)
	}
	relPath := filepath.Join(drive.Slug, subFolder)
	absPath := fmt.Sprintf("s3://%s/%s", s.Client.BucketName, key)
	parts := strings.Split(strings.Trim(subFolder, "/"), "/")
	name := parts[len(parts)-1]
	_, err = s.Store.CreateDirectoryMetadata(name, absPath, relPath, drive.ID.String())
	if err != nil {
		_ = s.Client.DeleteObject(context.Background(), s.Client.BucketName, key)
		return fmt.Errorf("s3manager: save directory metadata for %q: %w", key, err)
	}
	return nil
}

// s3DeleteBatchSize is the maximum number of object keys a single
// DeleteObjects request may carry.
const s3DeleteBatchSize = 1000

// deleteObjectsBatched deletes any number of keys by splitting them into
// protocol-sized batches.
func (s *S3Manager) deleteObjectsBatched(ctx context.Context, keys []string) error {
	for start := 0; start < len(keys); start += s3DeleteBatchSize {
		end := start + s3DeleteBatchSize
		if end > len(keys) {
			end = len(keys)
		}
		if err := s.Client.DeleteObjects(ctx, s.Client.BucketName, keys[start:end]); err != nil {
			return fmt.Errorf("batched delete of %d objects: %w", len(keys), err)
		}
	}
	return nil
}

// DeleteDir moves relPath's folder — and everything inside it — into the recycle
// bin, where it stays for the retention window before being permanently purged
// (mirroring DeleteFileV2 for folders). Every object under the folder prefix is
// copied under the recycle bin key prefix and deleted at its original key; the
// folder's metadata rows are soft-deleted in place so a restore can put them
// back unchanged.
func (s *S3Manager) DeleteDir(relPath, driveId, userId string) error {
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
	trimmed := strings.Trim(relPath, "/")
	if trimmed == "" {
		return errors.New("cannot delete the drive root")
	}
	pathKey := drive.Slug + "/" + trimmed
	rootRow, err := s.Store.GetDirectoryByDrivePathKey(drive.ID.String(), pathKey)
	if err != nil {
		return fmt.Errorf("s3manager: get directory metadata %q: %w", pathKey, err)
	}
	subtree, err := s.Store.ListFolderSubtreeUnscoped(drive.ID.String(), pathKey)
	if err != nil {
		return fmt.Errorf("s3manager: list folder subtree %q: %w", pathKey, err)
	}
	var sizeInMb float64
	for _, f := range subtree.Files {
		if !f.DeletedAt.Valid {
			sizeInMb += f.SizeInMb
		}
	}

	ctx := context.Background()
	keys, err := s.Client.ListObjects(ctx, s.Client.BucketName, pathKey+"/")
	if err != nil {
		return fmt.Errorf("s3manager: list prefix %q: %w", pathKey+"/", err)
	}

	recyclePrefix := archivus_constants.RecycleBinDirName + "/" + uuid.New().String()
	copiedKeys := make([]string, 0, len(keys))
	unwindCopies := func() {
		if err := s.deleteObjectsBatched(ctx, copiedKeys); err != nil {
			log.Warn().Err(err).Msg("s3manager: failed to roll back recycled copies after delete error")
		}
	}
	for _, k := range keys {
		dst := recyclePrefix + "/" + k
		if err := s.Client.CopyObject(ctx, s.Client.BucketName, k, dst); err != nil {
			unwindCopies()
			return fmt.Errorf("s3manager: copy %q to recycle bin: %w", k, err)
		}
		copiedKeys = append(copiedKeys, dst)
	}
	if len(keys) > 0 {
		if err := s.deleteObjectsBatched(ctx, keys); err != nil {
			s.copyRecycledBack(ctx, recyclePrefix, keys)
			return fmt.Errorf("s3manager: delete original objects under %q: %w", pathKey+"/", err)
		}
	}

	expiresAt := time.Now().AddDate(0, 0, archivus_constants.RecycleBinRetentionDays)
	err = s.Store.Transaction(func(tx *store.Store) error {
		if _, err := tx.CreateRecycleBinItem(rootRow.Name, rootRow.PathKey, rootRow.Prefix, recyclePrefix, "", "", driveId, userId, sizeInMb, expiresAt, true); err != nil {
			return err
		}
		return tx.HideFolderSubtree(driveId, pathKey)
	})
	if err != nil {
		// Put the originals back so nothing is silently lost.
		s.copyRecycledBack(ctx, recyclePrefix, keys)
		return fmt.Errorf("s3manager: record recycle bin item for folder %q: %w", pathKey, err)
	}
	return nil
}

// copyRecycledBack copies each recycled object back to its original key and
// removes the recycled copy. Best-effort rollback helper; failures are logged,
// never returned.
func (s *S3Manager) copyRecycledBack(ctx context.Context, recyclePrefix string, originalKeys []string) {
	recycledKeys := make([]string, 0, len(originalKeys))
	for _, k := range originalKeys {
		src := recyclePrefix + "/" + k
		if err := s.Client.CopyObject(ctx, s.Client.BucketName, src, k); err != nil {
			log.Warn().Err(err).Str("key", src).Msg("s3manager: failed to restore object during rollback")
			continue
		}
		recycledKeys = append(recycledKeys, src)
	}
	if err := s.deleteObjectsBatched(ctx, recycledKeys); err != nil {
		log.Warn().Err(err).Msg("s3manager: failed to remove recycled copies during rollback")
	}
}

// restoreFolder moves a recycled folder tree back to its original keys and
// reactivates its soft-deleted metadata rows.
func (s *S3Manager) restoreFolder(item models.RecycleBinItem) error {
	ctx := context.Background()
	rbPrefix := item.RecyclePathKey + "/"
	recycledKeys, err := s.Client.ListObjects(ctx, s.Client.BucketName, rbPrefix)
	if err != nil {
		return fmt.Errorf("s3manager: list recycled folder objects %q: %w", rbPrefix, err)
	}
	existing, err := s.Client.ListObjects(ctx, s.Client.BucketName, item.OriginalPathKey+"/")
	if err == nil && len(existing) > 0 {
		return errors.New("a folder already exists at the original location")
	}

	restoredKeys := make([]string, 0, len(recycledKeys))
	unwindRestores := func() {
		for _, orig := range restoredKeys {
			if rerr := s.Client.CopyObject(ctx, s.Client.BucketName, orig, rbPrefix+orig); rerr != nil {
				log.Warn().Err(rerr).Str("key", orig).Msg("s3manager: failed to re-recycle object after restore error")
			}
		}
	}
	for _, k := range recycledKeys {
		dst := strings.TrimPrefix(k, rbPrefix)
		if dst == "" || strings.HasSuffix(dst, "/") {
			continue // directory marker placeholders carry no content of their own
		}
		if err := s.Client.CopyObject(ctx, s.Client.BucketName, k, dst); err != nil {
			unwindRestores()
			return fmt.Errorf("s3manager: restore object to %q: %w", dst, err)
		}
		restoredKeys = append(restoredKeys, dst)
	}
	if len(recycledKeys) > 0 {
		if err := s.deleteObjectsBatched(ctx, recycledKeys); err != nil {
			log.Warn().Err(err).Msg("s3manager: failed to remove recycled objects after restore")
		}
	}

	if err := s.Store.UnhideFolderSubtree(item.DriveID.String(), item.OriginalPathKey, item.CreatedAt); err != nil {
		s.copyRecycledBackToBin(ctx, rbPrefix, restoredKeys)
		return fmt.Errorf("s3manager: restore folder metadata for %q: %w", item.OriginalPathKey, err)
	}
	if err := s.Store.DeleteRecycleBinItemByID(item.ID.String()); err != nil {
		return fmt.Errorf("s3manager: delete recycle bin item after restore: %w", err)
	}
	return nil
}

// copyRecycledBackToBin is the inverse of copyRecycledBack: best-effort return
// of already-restored objects into their recycle bin locations after a failed
// restore. Failures are logged, never returned.
func (s *S3Manager) copyRecycledBackToBin(ctx context.Context, rbPrefix string, restoredOriginalKeys []string) {
	recycledKeys := make([]string, 0, len(restoredOriginalKeys))
	for _, orig := range restoredOriginalKeys {
		dst := rbPrefix + orig
		if err := s.Client.CopyObject(ctx, s.Client.BucketName, orig, dst); err != nil {
			log.Warn().Err(err).Str("key", orig).Msg("s3manager: failed to re-recycle object after restore db error")
			continue
		}
		recycledKeys = append(recycledKeys, orig)
	}
	if err := s.deleteObjectsBatched(ctx, recycledKeys); err != nil {
		log.Warn().Err(err).Msg("s3manager: failed to remove restored originals during rollback")
	}
}

// purgeFolderItem permanently removes a recycled folder: every object under its
// recycle bin prefix, every thumbnail of the files that lived inside it, and
// their hidden metadata rows. Errors are logged with the cron logger.
func (s *S3Manager) purgeFolderItem(it models.RecycleBinItem) error {
	ctx := context.Background()
	thumbs, err := s.Store.ListHiddenFolderThumbnails(it.DriveID.String(), it.OriginalPathKey, it.CreatedAt)
	if err != nil {
		logging.CronErrorLogger.Error().Err(err).Str("path", it.OriginalPathKey).Msg("cron: purge: failed to list hidden folder thumbnails")
	} else {
		for _, thumb := range thumbs {
			if err := os.Remove(thumb); err != nil && !os.IsNotExist(err) {
				logging.CronErrorLogger.Error().Err(err).Str("path", thumb).Msg("cron: purge: failed to remove thumbnail")
			}
		}
	}
	keys, err := s.Client.ListObjects(ctx, s.Client.BucketName, it.RecyclePathKey+"/")
	if err != nil {
		logging.CronErrorLogger.Error().Err(err).Str("prefix", it.RecyclePathKey).Msg("cron: purge: failed to list recycled folder objects")
		return err
	}
	if len(keys) > 0 {
		if err := s.deleteObjectsBatched(ctx, keys); err != nil {
			logging.CronErrorLogger.Error().Err(err).Str("prefix", it.RecyclePathKey).Msg("cron: purge: failed to delete recycled folder objects")
			return err
		}
	}
	if err := s.Store.HardDeleteHiddenFolderSubtree(it.DriveID.String(), it.OriginalPathKey, it.CreatedAt); err != nil {
		logging.CronErrorLogger.Error().Err(err).Str("path", it.OriginalPathKey).Msg("cron: purge: failed to delete hidden folder rows")
	}
	return nil
}

func (s *S3Manager) CreateDirV2(subFolder, driveId, userId string) error {
	if subFolder == "" {
		return errors.New("subFolder cannot be empty")
	}
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
	trimmed := strings.Trim(subFolder, "/")
	// pathKey is the S3 key for this directory (trailing slash = S3 virtual dir convention).
	pathKey := drive.Slug + "/" + trimmed + "/"
	// prefix is the parent directory's key.
	// parent := filepath.Dir(trimmed)
	// var prefix string
	// if parent == "." {
	// 	prefix = drive.Slug + "/"
	// } else {
	// 	prefix = drive.Slug + "/" + parent + "/"
	// }
	if err := s.Client.CreateDirectory(context.Background(), s.Client.BucketName, pathKey); err != nil {
		return fmt.Errorf("s3manager: create directory %q in bucket %q: %w", pathKey, s.Client.BucketName, err)
	}
	_, err = s.EnsureDirectoryMetadata(userId, drive, strings.Split(trimmed, "/"))
	if err != nil {
		_ = s.Client.DeleteObject(context.Background(), s.Client.BucketName, pathKey)
		return fmt.Errorf("s3manager: save directory metadata for %q: %w", pathKey, err)
	}
	return nil
}

// DirExists reports whether relPath is an existing folder in the drive. S3 has
// no real directories, so the directory metadata rows (what listings are built
// from) are the source of truth.
func (s *S3Manager) DirExists(relPath, driveId, userId string) (bool, error) {
	hasAccess, err := s.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return false, err
	}
	if !hasAccess {
		return false, errors.New("user does not have write access to this drive")
	}
	trimmed := strings.Trim(relPath, "/")
	if trimmed == "" {
		return true, nil // drive root
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return false, fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	pathKey := drive.Slug + "/" + trimmed
	exists, err := s.Store.DirectoryExistsByDrivePathKey(drive.ID.String(), pathKey)
	if err != nil {
		return false, fmt.Errorf("s3manager: look up directory %q: %w", pathKey, err)
	}
	return exists, nil
}

// s3Key converts a DB relPath (drive.Slug/subpath) to an S3 object key (subpath).
func s3Key(relPath, driveSlug string) string {
	return strings.TrimPrefix(relPath, driveSlug+"/")
}
