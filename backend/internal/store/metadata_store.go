package store

import (
	"archivus/internal/models"
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Store) CreateDirectoryMetadata(name, absPath, relPath, driveID string) (models.DirectoryMetadata, error) {
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return models.DirectoryMetadata{}, fmt.Errorf("invalid drive ID: %w", err)
	}
	directoryMetadata := models.DirectoryMetadata{
		Name:    name,
		PathKey: relPath,
		DriveID: driveIDParsed,
	}
	result := s.conn().Create(&directoryMetadata)
	return directoryMetadata, result.Error
}

func (s *Store) CreateFileMetadata(name, relPath, dirPath, contentType, driveID, uploadedByID string, sizeInMb float64) (models.FileMetadata, error) {
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return models.FileMetadata{}, fmt.Errorf("invalid drive ID: %w", err)
	}
	uploadedByIDParsed, err := uuid.Parse(uploadedByID)
	if err != nil {
		return models.FileMetadata{}, fmt.Errorf("invalid uploaded by ID: %w", err)
	}
	fileMetadata := models.FileMetadata{
		Name:         name,
		PathKey:      relPath,
		Prefix:       dirPath,
		ContentType:  contentType,
		DriveID:      driveIDParsed,
		UploadedByID: uploadedByIDParsed,
		SizeInMb:     sizeInMb,
	}
	result := s.conn().Create(&fileMetadata)
	return fileMetadata, result.Error
}

func (s *Store) GetDirectoryMetadataByID(id string) (models.DirectoryMetadata, error) {
	var directoryMetadata models.DirectoryMetadata
	result := s.conn().First(&directoryMetadata, "id = ?", id)
	return directoryMetadata, result.Error
}

func (s *Store) GetFileMetadataByID(id string) (models.FileMetadata, error) {
	var fileMetadata models.FileMetadata
	result := s.conn().First(&fileMetadata, "id = ?", id)
	return fileMetadata, result.Error
}

func (s *Store) DeleteDirectoryMetadataByRelPath(relPath string) error {
	result := s.conn().Where("path_key = ?", relPath).Delete(&models.DirectoryMetadata{})
	return result.Error
}

func (s *Store) DeleteFileMetadataByRelPath(relPath string) error {
	result := s.conn().Where("path_key = ?", relPath).Delete(&models.FileMetadata{})
	return result.Error
}

// DeleteFileMetadataByID permanently removes a file metadata row. Used when a
// file is moved into the recycle bin (the RecycleBinItem row then owns it).
func (s *Store) DeleteFileMetadataByID(id string) error {
	result := s.conn().Unscoped().Where("id = ?", id).Delete(&models.FileMetadata{})
	return result.Error
}

// UpdateFileMetadataLocation moves a file row to a new key/prefix after its
// bytes have been relocated on disk/S3.
func (s *Store) UpdateFileMetadataLocation(id, pathKey, prefix string) error {
	result := s.conn().Model(&models.FileMetadata{}).Where("id = ?", id).Updates(map[string]interface{}{
		"path_key": pathKey,
		"prefix":   prefix,
	})
	return result.Error
}

func (s *Store) ListFilesByRelPath(dirAbsPath string) ([]models.FileMetadata, error) {
	var files []models.FileMetadata
	result := s.conn().Where("path_key LIKE ?", dirAbsPath+"/%").Find(&files)
	if result.Error != nil {
		return nil, result.Error
	}
	return files, nil
}

func (s *Store) GetFileMetadataByRelPath(relPath string) (models.FileMetadata, error) {
	var fileMetadata models.FileMetadata
	result := s.conn().Where("rel_path = ?", relPath).First(&fileMetadata)
	return fileMetadata, result.Error
}

func (s *Store) UpdateFileMetadataPaths(id, absPath, s3Key, relPath, dirPath string) error {
	result := s.conn().Model(&models.FileMetadata{}).Where("id = ?", id).Updates(map[string]interface{}{
		"abs_path": absPath,
		"s3_key":   s3Key,
		"rel_path": relPath,
		"dir_path": dirPath,
	})
	return result.Error
}

// V2 methods: PathKey and Prefix are stored correctly.
// PathKey = the object/file key (S3 key or absolute disk path).
// Prefix  = the parent directory key with trailing slash (S3) or absolute dir path (disk).

func (s *Store) CreateFileMetadataV2(name, pathKey, prefix, contentType, driveID, uploadedByID string, sizeInMb float64) (models.FileMetadata, error) {
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return models.FileMetadata{}, fmt.Errorf("invalid drive ID: %w", err)
	}
	uploadedByIDParsed, err := uuid.Parse(uploadedByID)
	if err != nil {
		return models.FileMetadata{}, fmt.Errorf("invalid uploaded by ID: %w", err)
	}
	fm := models.FileMetadata{
		Name:         name,
		PathKey:      pathKey,
		Prefix:       prefix,
		ContentType:  contentType,
		DriveID:      driveIDParsed,
		UploadedByID: uploadedByIDParsed,
		SizeInMb:     sizeInMb,
	}
	result := s.conn().Create(&fm)
	return fm, result.Error
}

// CreatePendingFileMetadataV2 records a file whose bytes have been assembled
// locally but not yet pushed to the storage backend. sourcePath is the local
// assembled file the background worker will upload. The row is visible in
// listings immediately (as "pending") so users see the file is on its way.
func (s *Store) CreatePendingFileMetadataV2(name, pathKey, prefix, contentType, driveID, uploadedByID string, sizeInMb float64, sourcePath string) (models.FileMetadata, error) {
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return models.FileMetadata{}, fmt.Errorf("invalid drive ID: %w", err)
	}
	uploadedByIDParsed, err := uuid.Parse(uploadedByID)
	if err != nil {
		return models.FileMetadata{}, fmt.Errorf("invalid uploaded by ID: %w", err)
	}
	fm := models.FileMetadata{
		Name:              name,
		PathKey:           pathKey,
		Prefix:            prefix,
		ContentType:       contentType,
		DriveID:           driveIDParsed,
		UploadedByID:      uploadedByIDParsed,
		SizeInMb:          sizeInMb,
		UploadStatus:      models.UploadStatusPending,
		PendingSourcePath: sourcePath,
	}
	result := s.conn().Create(&fm)
	return fm, result.Error
}

// MarkFileMetadataPending flips an existing (ready) file back to pending so a
// chunked overwrite can replace its bytes. The old size/content-type are left
// intact so the listing keeps showing the current version until the new bytes land.
func (s *Store) MarkFileMetadataPending(id, sourcePath string) error {
	result := s.conn().Model(&models.FileMetadata{}).Where("id = ?", id).Updates(map[string]interface{}{
		"upload_status":       models.UploadStatusPending,
		"pending_source_path": sourcePath,
		"upload_attempts":     0,
	})
	return result.Error
}

// GetPendingFileUploads returns files awaiting a background push to storage,
// oldest first, so the worker drains them in arrival order.
func (s *Store) GetPendingFileUploads(limit int) ([]models.FileMetadata, error) {
	var files []models.FileMetadata
	result := s.conn().Where("upload_status = ?", models.UploadStatusPending).
		Order("updated_at ASC").Limit(limit).Find(&files)
	return files, result.Error
}

// ClaimPendingUpload atomically transitions a row from pending to uploading and
// bumps its attempt count. It returns claimed=true only if this call is the one
// that won the row, so the inline finalizer and the cron recovery job never
// upload the same file twice.
func (s *Store) ClaimPendingUpload(id string) (bool, error) {
	result := s.conn().Model(&models.FileMetadata{}).
		Where("id = ? AND upload_status = ?", id, models.UploadStatusPending).
		Updates(map[string]interface{}{
			"upload_status":   models.UploadStatusUploading,
			"upload_attempts": gorm.Expr("upload_attempts + 1"),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// MarkFileUploadReady marks a finalized upload as ready, records its final size,
// clears the staging pointer, and clears the thumbnail path so the thumbnail
// job regenerates it from the freshly uploaded content.
func (s *Store) MarkFileUploadReady(id string, sizeInMb float64) error {
	result := s.conn().Model(&models.FileMetadata{}).Where("id = ?", id).Updates(map[string]interface{}{
		"upload_status":       models.UploadStatusReady,
		"pending_source_path": "",
		"size_in_mb":          sizeInMb,
		"thumbnail_path":      "",
	})
	return result.Error
}

// RevertUploadToPending puts a row that failed to finalize back into the pending
// queue so a later worker pass retries it (used for transient errors, up to
// MaxUploadAttempts).
func (s *Store) RevertUploadToPending(id string) error {
	result := s.conn().Model(&models.FileMetadata{}).Where("id = ?", id).
		Update("upload_status", models.UploadStatusPending)
	return result.Error
}

// MarkFileUploadFailed gives up on a pending upload after too many attempts. The
// staging file pointer is kept for diagnostics.
func (s *Store) MarkFileUploadFailed(id string) error {
	result := s.conn().Model(&models.FileMetadata{}).Where("id = ?", id).
		Update("upload_status", models.UploadStatusFailed)
	return result.Error
}

func (s *Store) CreateDirectoryMetadataV2(name, pathKey, prefix, driveID string) (models.DirectoryMetadata, error) {
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return models.DirectoryMetadata{}, fmt.Errorf("invalid drive ID: %w", err)
	}
	dm := models.DirectoryMetadata{
		Name:    name,
		PathKey: pathKey,
		Prefix:  prefix,
		DriveID: driveIDParsed,
	}
	result := s.conn().Create(&dm)
	return dm, result.Error
}

// GetFileMetadataByDrivePathKey looks up a single file by its exact key within a
// drive. Returns ErrRecordNotFound when no such file exists, which upload paths
// use to distinguish a first upload from an overwrite/new-version.
func (s *Store) GetFileMetadataByDrivePathKey(driveID, pathKey string) (models.FileMetadata, error) {
	var fm models.FileMetadata
	result := s.conn().Where("drive_id = ? AND path_key = ?", driveID, pathKey).First(&fm)
	return fm, result.Error
}

// UpdateFileMetadataContent updates a file row in place after its bytes were
// overwritten (home mode) or replaced by a newer version (biz mode). The
// thumbnail path is cleared so the thumbnail job regenerates it from the new
// content.
func (s *Store) UpdateFileMetadataContent(id string, sizeInMb float64, contentType string) error {
	result := s.conn().Model(&models.FileMetadata{}).Where("id = ?", id).Updates(map[string]interface{}{
		"size_in_mb":     sizeInMb,
		"content_type":   contentType,
		"thumbnail_path": "",
	})
	return result.Error
}

// DirectoryExistsByDrivePathKey reports whether a directory row exists for the
// exact key within a drive. Upload paths use it to reject a write into a folder
// that was never created.
func (s *Store) DirectoryExistsByDrivePathKey(driveID, pathKey string) (bool, error) {
	var count int64
	result := s.conn().Model(&models.DirectoryMetadata{}).Where("drive_id = ? AND path_key = ?", driveID, pathKey).Count(&count)
	return count > 0, result.Error
}

func (s *Store) GetFileMetadataByDirPrefix(driveID string, prefixes [2]string) ([]models.FileMetadata, error) {
	var files []models.FileMetadata
	result := s.conn().Where("drive_id = ? AND prefix IN ?", driveID, prefixes).Find(&files)
	return files, result.Error
}

func (s *Store) GetDirectoriesByParentPrefix(driveID string, prefixes [2]string) ([]models.DirectoryMetadata, error) {
	var dirs []models.DirectoryMetadata
	result := s.conn().Where("drive_id = ? AND prefix IN ?", driveID, prefixes).Find(&dirs)
	return dirs, result.Error
}

// CountFileMetadataByDirPrefix returns the number of files directly under the
// given directory prefixes, used to page the combined file/directory listing.
func (s *Store) CountFileMetadataByDirPrefix(driveID string, prefixes [2]string) (int64, error) {
	var count int64
	result := s.conn().Model(&models.FileMetadata{}).Where("drive_id = ? AND prefix IN ?", driveID, prefixes).Count(&count)
	return count, result.Error
}

// CountDirectoriesByParentPrefix returns the number of directories directly
// under the given parent prefixes.
func (s *Store) CountDirectoriesByParentPrefix(driveID string, prefixes [2]string) (int64, error) {
	var count int64
	result := s.conn().Model(&models.DirectoryMetadata{}).Where("drive_id = ? AND prefix IN ?", driveID, prefixes).Count(&count)
	return count, result.Error
}

// GetFileMetadataByDirPrefixPaged returns files under the given prefixes ordered
// by name, sliced by limit/offset. A negative limit disables the limit.
func (s *Store) GetFileMetadataByDirPrefixPaged(driveID string, prefixes [2]string, limit, offset int) ([]models.FileMetadata, error) {
	var files []models.FileMetadata
	result := s.conn().Where("drive_id = ? AND prefix IN ?", driveID, prefixes).
		Order("name ASC").Limit(limit).Offset(offset).Find(&files)
	return files, result.Error
}

// GetDirectoriesByParentPrefixPaged returns directories under the given prefixes
// ordered by name, sliced by limit/offset. A negative limit disables the limit.
func (s *Store) GetDirectoriesByParentPrefixPaged(driveID string, prefixes [2]string, limit, offset int) ([]models.DirectoryMetadata, error) {
	var dirs []models.DirectoryMetadata
	result := s.conn().Where("drive_id = ? AND prefix IN ?", driveID, prefixes).
		Order("name ASC").Limit(limit).Offset(offset).Find(&dirs)
	return dirs, result.Error
}

func (s *Store) GetFileMetadatasWoThumbnails(ctx context.Context, limit int) ([]models.FileMetadata, error) {
	var files []models.FileMetadata
	result := s.conn().Where("(thumbnail_path IS NULL OR thumbnail_path = '') AND upload_status = ? AND is_image = true", models.UploadStatusReady).Limit(limit).Find(&files)
	return files, result.Error
}

func (s *Store) UpdateFileMetadataThumbnailPath(id, thumbnailPath string) error {
	result := s.conn().Model(&models.FileMetadata{}).Where("id = ?", id).Update("thumbnail_path", thumbnailPath)
	return result.Error
}

func (s *Store) GetFileMetadatasWoExtension(limit int) ([]models.FileMetadata, error) {
	var files []models.FileMetadata
	result := s.conn().Where("extension = ''").Limit(limit).Find(&files)
	return files, result.Error
}

// MarkFileMetadatasAsImages flags the given file rows as images in a single
// query.
func (s *Store) MarkFileMetadatasAsImages(ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	result := s.conn().Model(&models.FileMetadata{}).Where("id IN ?", ids).Update("is_image", true)
	return result.Error
}

// UpdateFileMetadataExtensions sets the extension for many file rows in a
// single query using a CASE expression keyed by file ID.
func (s *Store) UpdateFileMetadataExtensions(extensionsByID map[uuid.UUID]string) error {
	if len(extensionsByID) == 0 {
		return nil
	}
	caseSQL := "CASE id "
	args := make([]interface{}, 0, len(extensionsByID)*2)
	ids := make([]uuid.UUID, 0, len(extensionsByID))
	for id, ext := range extensionsByID {
		caseSQL += "WHEN ? THEN ? "
		args = append(args, id, ext)
		ids = append(ids, id)
	}
	caseSQL += "END"
	result := s.conn().Model(&models.FileMetadata{}).
		Where("id IN ?", ids).
		Update("extension", gorm.Expr(caseSQL, args...))
	return result.Error
}
