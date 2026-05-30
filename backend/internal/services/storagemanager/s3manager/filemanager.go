package s3manager

import (
	"archivus/internal/models"
	storage_types "archivus/internal/services/storagemanager/types"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *S3Manager) UploadFile(relPath, driveId, userId string, file multipart.File, fileHeader *multipart.FileHeader) error {
	hasAccess, err := s.checkUserDriveWriteAccess(userId, driveId)
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
	dirKey := strings.Trim(s3Key(relPath, drive.Slug), "/")
	key := filepath.Join(dirKey, fileHeader.Filename)
	contentType := fileHeader.Header.Get("Content-Type")
	if err := s.Client.PutObject(context.Background(), drive.Slug, key, contentType, fileHeader.Size, file); err != nil {
		return fmt.Errorf("s3manager: upload %q: %w", key, err)
	}
	dbRelPath := filepath.Join(drive.Slug, key)
	dbAbsPath := fmt.Sprintf("s3://%s/%s", drive.Slug, key)
	dbDirPath := fmt.Sprintf("s3://%s/%s", drive.Slug, filepath.Dir(key))
	sizeInMb := float64(fileHeader.Size) / (1 << 20)
	_, err = s.Store.CreateFileMetadata(fileHeader.Filename, dbAbsPath, dbRelPath, dbDirPath, driveId, userId, sizeInMb)
	if err != nil {
		_ = s.Client.DeleteObject(context.Background(), drive.Slug, key)
		return fmt.Errorf("s3manager: save file metadata for %q: %w", key, err)
	}
	return nil
}

func (s *S3Manager) MoveFile(srcRelPath, dstRelPath, driveId, userId string) error {
	hasAccess, err := s.checkUserDriveWriteAccess(userId, driveId)
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
	srcKey := s3Key(srcRelPath, drive.Slug)
	dstKey := s3Key(dstRelPath, drive.Slug)
	md, err := s.Store.GetFileMetadataByRelPath(filepath.Join(drive.Slug, srcRelPath))
	if err != nil {
		return fmt.Errorf("s3manager: get metadata for %q: %w", srcRelPath, err)
	}
	ctx := context.Background()
	if err := s.Client.CopyObject(ctx, drive.Slug, srcKey, dstKey); err != nil {
		return fmt.Errorf("s3manager: copy %q to %q: %w", srcKey, dstKey, err)
	}
	if err := s.Client.DeleteObject(ctx, drive.Slug, srcKey); err != nil {
		_ = s.Client.DeleteObject(ctx, drive.Slug, dstKey)
		return fmt.Errorf("s3manager: delete source %q after move: %w", srcKey, err)
	}
	newRelPath := filepath.Join(drive.Slug, dstRelPath)
	newAbsPath := fmt.Sprintf("s3://%s/%s", drive.Slug, dstKey)
	newDirPath := fmt.Sprintf("s3://%s/%s", drive.Slug, filepath.Dir(dstKey))
	return s.Store.UpdateFileMetadataPaths(md.ID, newAbsPath, newRelPath, newDirPath)
}

func (s *S3Manager) DownloadFile(fileId string, driveId, userId string) (*os.File, *models.FileMetadata, error) {
	hasAccess, err := s.checkUserHasDriveAccess(userId, driveId)
	if err != nil {
		return nil, nil, err
	}
	if !hasAccess {
		return nil, nil, errors.New("user does not have access to this drive")
	}
	md, err := s.Store.GetFileMetadataByID(fileId)
	if err != nil {
		return nil, nil, fmt.Errorf("s3manager: get file metadata %d: %w", fileId, err)
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return nil, nil, fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	key := s3Key(md.RelPath, drive.Slug)
	out, err := s.Client.GetObject(context.Background(), drive.Slug, key)
	if err != nil {
		return nil, nil, fmt.Errorf("s3manager: get object %q: %w", key, err)
	}
	defer out.Body.Close()

	tmp, err := os.CreateTemp("", "archivus-download-*")
	if err != nil {
		return nil, nil, fmt.Errorf("s3manager: create temp file: %w", err)
	}
	if _, err := io.Copy(tmp, out.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, nil, fmt.Errorf("s3manager: write to temp file: %w", err)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, nil, fmt.Errorf("s3manager: seek temp file: %w", err)
	}
	return tmp, &md, nil
}

func (s *S3Manager) GetFiles(relPath, driveId, userId string) ([]storage_types.DirEntry, error) {
	hasAccess, err := s.checkUserHasDriveAccess(userId, driveId)
	if err != nil {
		return nil, err
	}
	if !hasAccess {
		return nil, errors.New("user does not have access to this drive")
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return nil, fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	prefix := strings.Trim(s3Key(relPath, drive.Slug), "/")
	if prefix != "" {
		prefix += "/"
	}
	ctx := context.Background()
	entries, err := s.Client.ListObjectsOnelevel(ctx, drive.Slug, prefix)
	if err != nil {
		return nil, fmt.Errorf("s3manager: list %q: %w", prefix, err)
	}
	var dirEntries []storage_types.DirEntry
	for i, e := range entries {
		name := filepath.Base(strings.TrimSuffix(e.Key, "/"))
		signedURL := ""
		if !e.IsDir {
			signedURL, _ = s.Client.PresignGetObject(ctx, drive.Slug, e.Key, 15*time.Minute)
		}
		dirEntries = append(dirEntries, storage_types.DirEntry{
			ID:             uint(i),
			Name:           name,
			IsDir:          e.IsDir,
			Extension:      filepath.Ext(name),
			SignedUrl:      signedURL,
			Path:           filepath.Join(drive.Slug, e.Key),
			NavigationPath: filepath.Join(relPath, name),
		})
	}
	return dirEntries, nil
}
