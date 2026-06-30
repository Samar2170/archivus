package s3manager

import (
	storage_types "archivus/internal/services/storagemanager/types"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

// objectKey builds the S3 key for a drive-relative path. Root ("") maps to the
// drive slug. Keys never carry a trailing slash (the file/dir convention used
// by UploadFileV2 / EnsureDirectoryMetadata).
func objectKey(slug, relPath string) string {
	trimmed := strings.Trim(relPath, "/")
	if trimmed == "" {
		return slug
	}
	return slug + "/" + trimmed
}

// tempFile wraps a downloaded temp file so it is deleted from disk on Close.
type tempFile struct {
	*os.File
}

func (t *tempFile) Close() error {
	name := t.File.Name()
	err := t.File.Close()
	os.Remove(name)
	return err
}

func (s *S3Manager) StatPath(relPath, driveId, userId string) (*storage_types.StatInfo, error) {
	hasAccess, err := s.CheckUserHasDriveAccess(userId, driveId)
	if err != nil {
		return nil, err
	}
	if !hasAccess {
		return nil, os.ErrPermission
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return nil, fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	if strings.Trim(relPath, "/") == "" {
		return &storage_types.StatInfo{Name: drive.Slug, IsDir: true}, nil
	}
	key := objectKey(drive.Slug, relPath)
	ctx := context.Background()
	// Try as a file first.
	if head, err := s.Client.HeadObject(ctx, s.Client.BucketName, key); err == nil {
		var size int64
		if head.ContentLength != nil {
			size = *head.ContentLength
		}
		info := &storage_types.StatInfo{Name: path.Base(key), IsDir: false, Size: size}
		if head.LastModified != nil {
			info.ModTime = *head.LastModified
		}
		return info, nil
	}
	// Fall back to a directory lookup in the metadata store.
	if _, err := s.Store.GetDirectoryMetadataByPathKey(drive.ID.String(), key); err == nil {
		return &storage_types.StatInfo{Name: path.Base(key), IsDir: true}, nil
	}
	return nil, os.ErrNotExist
}

func (s *S3Manager) ReadFile(relPath, driveId, userId string) (io.ReadSeekCloser, *storage_types.StatInfo, error) {
	hasAccess, err := s.CheckUserHasDriveAccess(userId, driveId)
	if err != nil {
		return nil, nil, err
	}
	if !hasAccess {
		return nil, nil, os.ErrPermission
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return nil, nil, fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	key := objectKey(drive.Slug, relPath)
	ctx := context.Background()
	out, err := s.Client.GetObject(ctx, s.Client.BucketName, key)
	if err != nil {
		return nil, nil, os.ErrNotExist
	}
	defer out.Body.Close()

	tmp, err := os.CreateTemp("", "archivus-webdav-*")
	if err != nil {
		return nil, nil, fmt.Errorf("s3manager: create temp file: %w", err)
	}
	if _, err := io.Copy(tmp, out.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, nil, fmt.Errorf("s3manager: download %q: %w", key, err)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, nil, fmt.Errorf("s3manager: seek temp file: %w", err)
	}
	info := &storage_types.StatInfo{Name: path.Base(key), IsDir: false}
	if out.ContentLength != nil {
		info.Size = *out.ContentLength
	}
	if out.LastModified != nil {
		info.ModTime = *out.LastModified
	}
	return &tempFile{tmp}, info, nil
}

func (s *S3Manager) WriteFileStream(relPath, driveId, userId string, r io.Reader, size int64, contentType string) error {
	hasAccess, err := s.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return os.ErrPermission
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	pathKey := objectKey(drive.Slug, relPath)
	prefix := path.Dir(pathKey) + "/"
	ctx := context.Background()
	if err := s.Client.PutObject(ctx, s.Client.BucketName, pathKey, contentType, size, r); err != nil {
		return fmt.Errorf("s3manager: upload %q: %w", pathKey, err)
	}
	sizeInMb := float64(size) / (1 << 20)
	existing, err := s.Store.GetFileMetadataByPathKey(drive.ID.String(), pathKey)
	if err == nil {
		return s.Store.UpdateFileMetadataByID(existing.ID.String(), map[string]interface{}{
			"size_in_mb":   sizeInMb,
			"content_type": contentType,
		})
	}
	if _, err := s.Store.CreateFileMetadataV2(path.Base(pathKey), pathKey, prefix, contentType, driveId, userId, sizeInMb); err != nil {
		_ = s.Client.DeleteObject(ctx, s.Client.BucketName, pathKey)
		return fmt.Errorf("s3manager: save file metadata for %q: %w", pathKey, err)
	}
	return nil
}

func (s *S3Manager) Remove(relPath, driveId, userId string) error {
	hasAccess, err := s.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return os.ErrPermission
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	if strings.Trim(relPath, "/") == "" {
		return errors.New("s3manager: refusing to remove drive root")
	}
	key := objectKey(drive.Slug, relPath)
	ctx := context.Background()
	// A single object => file delete.
	if _, err := s.Client.HeadObject(ctx, s.Client.BucketName, key); err == nil {
		if err := s.Client.DeleteObject(ctx, s.Client.BucketName, key); err != nil {
			return fmt.Errorf("s3manager: delete %q: %w", key, err)
		}
		return s.Store.DeleteSubtreeMetadata(drive.ID.String(), key)
	}
	// Otherwise treat as a directory: delete the whole prefix.
	keys, err := s.Client.ListObjects(ctx, s.Client.BucketName, key+"/")
	if err != nil {
		return fmt.Errorf("s3manager: list %q: %w", key, err)
	}
	if len(keys) > 0 {
		if err := s.Client.DeleteObjects(ctx, s.Client.BucketName, keys); err != nil {
			return fmt.Errorf("s3manager: delete prefix %q: %w", key, err)
		}
	}
	return s.Store.DeleteSubtreeMetadata(drive.ID.String(), key)
}

func (s *S3Manager) Rename(oldRelPath, newRelPath, driveId, userId string) error {
	hasAccess, err := s.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return os.ErrPermission
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	if strings.Trim(oldRelPath, "/") == "" || strings.Trim(newRelPath, "/") == "" {
		return errors.New("s3manager: cannot rename drive root")
	}
	oldKey := objectKey(drive.Slug, oldRelPath)
	newKey := objectKey(drive.Slug, newRelPath)
	ctx := context.Background()

	// File rename: single object copy + delete.
	if _, err := s.Client.HeadObject(ctx, s.Client.BucketName, oldKey); err == nil {
		if err := s.Client.CopyObject(ctx, s.Client.BucketName, oldKey, newKey); err != nil {
			return fmt.Errorf("s3manager: copy %q to %q: %w", oldKey, newKey, err)
		}
		if err := s.Client.DeleteObject(ctx, s.Client.BucketName, oldKey); err != nil {
			return fmt.Errorf("s3manager: delete source %q: %w", oldKey, err)
		}
		return s.renameMetadata(drive.ID.String(), oldKey, newKey)
	}

	// Directory rename: copy every object under the prefix, then delete originals.
	srcKeys, err := s.Client.ListObjects(ctx, s.Client.BucketName, oldKey+"/")
	if err != nil {
		return fmt.Errorf("s3manager: list %q: %w", oldKey, err)
	}
	if len(srcKeys) == 0 {
		return os.ErrNotExist
	}
	for _, sk := range srcKeys {
		dk := newKey + strings.TrimPrefix(sk, oldKey)
		if err := s.Client.CopyObject(ctx, s.Client.BucketName, sk, dk); err != nil {
			return fmt.Errorf("s3manager: copy %q to %q: %w", sk, dk, err)
		}
	}
	if err := s.Client.DeleteObjects(ctx, s.Client.BucketName, srcKeys); err != nil {
		return fmt.Errorf("s3manager: delete sources under %q: %w", oldKey, err)
	}
	return s.renameMetadata(drive.ID.String(), oldKey, newKey)
}

// renameMetadata rewrites PathKey/Prefix/Name for the moved node and descendants.
// Files store Prefix with a trailing slash; directories store it without.
func (s *S3Manager) renameMetadata(driveID, oldKey, newKey string) error {
	files, err := s.Store.ListFileMetadataBySubtree(driveID, oldKey)
	if err != nil {
		return fmt.Errorf("s3manager: list files for rename: %w", err)
	}
	for _, f := range files {
		np := newKey + strings.TrimPrefix(f.PathKey, oldKey)
		if err := s.Store.UpdateFileMetadataByID(f.ID.String(), map[string]interface{}{
			"path_key": np,
			"prefix":   path.Dir(np) + "/",
			"name":     path.Base(np),
		}); err != nil {
			return fmt.Errorf("s3manager: update file metadata for rename: %w", err)
		}
	}
	dirs, err := s.Store.ListDirectoryMetadataBySubtree(driveID, oldKey)
	if err != nil {
		return fmt.Errorf("s3manager: list dirs for rename: %w", err)
	}
	for _, d := range dirs {
		np := newKey + strings.TrimPrefix(d.PathKey, oldKey)
		if err := s.Store.UpdateDirectoryMetadataByID(d.ID.String(), map[string]interface{}{
			"path_key": np,
			"prefix":   path.Dir(np),
			"name":     path.Base(np),
		}); err != nil {
			return fmt.Errorf("s3manager: update dir metadata for rename: %w", err)
		}
	}
	return nil
}
