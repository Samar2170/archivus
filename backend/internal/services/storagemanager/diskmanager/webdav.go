package diskmanager

import (
	storage_types "archivus/internal/services/storagemanager/types"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// abs resolves a drive-relative path to its absolute path on disk.
func (dm *DiskManager) abs(slug, relPath string) string {
	return filepath.Join(dm.Home, slug, strings.Trim(relPath, "/"))
}

func (dm *DiskManager) StatPath(relPath, driveId, userId string) (*storage_types.StatInfo, error) {
	hasAccess, err := dm.CheckUserHasDriveAccess(userId, driveId)
	if err != nil {
		return nil, err
	}
	if !hasAccess {
		return nil, os.ErrPermission
	}
	drive, err := dm.Store.GetDriveByID(driveId)
	if err != nil {
		return nil, fmt.Errorf("diskmanager: get drive by id %q: %w", driveId, err)
	}
	abs := dm.abs(drive.Slug, relPath)
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	name := info.Name()
	if strings.Trim(relPath, "/") == "" {
		name = drive.Slug
	}
	return &storage_types.StatInfo{
		Name:    name,
		IsDir:   info.IsDir(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}, nil
}

func (dm *DiskManager) ReadFile(relPath, driveId, userId string) (io.ReadSeekCloser, *storage_types.StatInfo, error) {
	hasAccess, err := dm.CheckUserHasDriveAccess(userId, driveId)
	if err != nil {
		return nil, nil, err
	}
	if !hasAccess {
		return nil, nil, os.ErrPermission
	}
	drive, err := dm.Store.GetDriveByID(driveId)
	if err != nil {
		return nil, nil, fmt.Errorf("diskmanager: get drive by id %q: %w", driveId, err)
	}
	abs := dm.abs(drive.Slug, relPath)
	f, err := os.Open(abs)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return f, &storage_types.StatInfo{
		Name:    info.Name(),
		IsDir:   info.IsDir(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}, nil
}

func (dm *DiskManager) WriteFileStream(relPath, driveId, userId string, r io.Reader, size int64, contentType string) error {
	hasAccess, err := dm.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return os.ErrPermission
	}
	drive, err := dm.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("diskmanager: get drive by id %q: %w", driveId, err)
	}
	pathKey := dm.abs(drive.Slug, relPath)
	prefix := filepath.Dir(pathKey)
	if err := os.MkdirAll(prefix, 0755); err != nil {
		return fmt.Errorf("diskmanager: create parent dir %q: %w", prefix, err)
	}
	outFile, err := os.Create(pathKey)
	if err != nil {
		return fmt.Errorf("diskmanager: create file %q: %w", pathKey, err)
	}
	written, copyErr := io.Copy(outFile, r)
	if closeErr := outFile.Close(); closeErr != nil && copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		os.Remove(pathKey)
		return fmt.Errorf("diskmanager: write file %q: %w", pathKey, copyErr)
	}
	sizeInMb := float64(written) / (1 << 20)
	name := filepath.Base(pathKey)

	existing, err := dm.Store.GetFileMetadataByPathKey(drive.ID.String(), pathKey)
	if err == nil {
		return dm.Store.UpdateFileMetadataByID(existing.ID.String(), map[string]interface{}{
			"size_in_mb":   sizeInMb,
			"content_type": contentType,
		})
	}
	if _, err := dm.Store.CreateFileMetadataV2(name, pathKey, prefix, contentType, driveId, userId, sizeInMb); err != nil {
		os.Remove(pathKey)
		return fmt.Errorf("diskmanager: create file metadata for %q: %w", pathKey, err)
	}
	return nil
}

func (dm *DiskManager) Remove(relPath, driveId, userId string) error {
	hasAccess, err := dm.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return os.ErrPermission
	}
	drive, err := dm.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("diskmanager: get drive by id %q: %w", driveId, err)
	}
	if strings.Trim(relPath, "/") == "" {
		return errors.New("diskmanager: refusing to remove drive root")
	}
	pathKey := dm.abs(drive.Slug, relPath)
	if _, err := os.Stat(pathKey); err != nil {
		return err
	}
	if err := os.RemoveAll(pathKey); err != nil {
		return fmt.Errorf("diskmanager: remove %q: %w", pathKey, err)
	}
	return dm.Store.DeleteSubtreeMetadata(drive.ID.String(), pathKey)
}

func (dm *DiskManager) Rename(oldRelPath, newRelPath, driveId, userId string) error {
	hasAccess, err := dm.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return os.ErrPermission
	}
	drive, err := dm.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("diskmanager: get drive by id %q: %w", driveId, err)
	}
	if strings.Trim(oldRelPath, "/") == "" || strings.Trim(newRelPath, "/") == "" {
		return errors.New("diskmanager: cannot rename drive root")
	}
	oldKey := dm.abs(drive.Slug, oldRelPath)
	newKey := dm.abs(drive.Slug, newRelPath)
	if _, err := os.Stat(oldKey); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(newKey), 0755); err != nil {
		return fmt.Errorf("diskmanager: create parent dir for %q: %w", newKey, err)
	}
	if err := os.Rename(oldKey, newKey); err != nil {
		return fmt.Errorf("diskmanager: rename %q to %q: %w", oldKey, newKey, err)
	}
	return dm.renameMetadata(drive.ID.String(), oldKey, newKey)
}

// renameMetadata rewrites PathKey/Prefix/Name for the moved node and every descendant.
func (dm *DiskManager) renameMetadata(driveID, oldKey, newKey string) error {
	files, err := dm.Store.ListFileMetadataBySubtree(driveID, oldKey)
	if err != nil {
		return fmt.Errorf("diskmanager: list files for rename: %w", err)
	}
	for _, f := range files {
		np := newKey + strings.TrimPrefix(f.PathKey, oldKey)
		if err := dm.Store.UpdateFileMetadataByID(f.ID.String(), map[string]interface{}{
			"path_key": np,
			"prefix":   filepath.Dir(np),
			"name":     filepath.Base(np),
		}); err != nil {
			return fmt.Errorf("diskmanager: update file metadata for rename: %w", err)
		}
	}
	dirs, err := dm.Store.ListDirectoryMetadataBySubtree(driveID, oldKey)
	if err != nil {
		return fmt.Errorf("diskmanager: list dirs for rename: %w", err)
	}
	for _, d := range dirs {
		np := newKey + strings.TrimPrefix(d.PathKey, oldKey)
		if err := dm.Store.UpdateDirectoryMetadataByID(d.ID.String(), map[string]interface{}{
			"path_key": np,
			"prefix":   filepath.Dir(np),
			"name":     filepath.Base(np),
		}); err != nil {
			return fmt.Errorf("diskmanager: update dir metadata for rename: %w", err)
		}
	}
	return nil
}
