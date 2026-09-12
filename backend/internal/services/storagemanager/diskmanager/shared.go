package diskmanager

import (
	"archivus/internal/models"
	storage_types "archivus/internal/services/storagemanager/types"
	"archivus/internal/utils"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// GetSharedFiles lists a folder inside a shared subtree for a user whose access
// comes from a folder share rather than drive membership. rootRelPath must
// match an existing share for the user; relPath must stay within it.
func (dm *DiskManager) GetSharedFiles(rootRelPath, relPath, driveId, userId string, page, pageSize int, query storage_types.ListFilesQuery) (storage_types.PagedDirEntries, error) {
	var out storage_types.PagedDirEntries
	rootRelPath = utils.NormalizeRelPath(rootRelPath)
	relPath = utils.NormalizeRelPath(relPath)
	if !utils.PathWithinRoot(rootRelPath, relPath) {
		return out, errors.New("requested path is outside the shared folder")
	}
	if _, err := dm.CheckUserHasSharedFolderAccess(userId, driveId, rootRelPath); err != nil {
		return out, err
	}
	drive, err := dm.Store.GetDriveByID(driveId)
	if err != nil {
		return out, fmt.Errorf("diskmanager: get drive by id %q: %w", driveId, err)
	}
	// RootPathKey is drive-relative; re-derive the absolute directory prefixes
	// this backend lists by.
	dirAbs := filepath.Join(dm.Home, drive.Slug, filepath.FromSlash(relPath))
	dirPrefixes := [2]string{dirAbs, dirAbs + "/"}
	return dm.listAtPrefixes(relPath, drive.ID.String(), dirPrefixes, page, pageSize, query)
}

// DownloadSharedFile serves a file from inside a shared subtree. The file must
// exist, be ready, and live under the shared root.
func (dm *DiskManager) DownloadSharedFile(fileId, rootRelPath, driveId, userId string) (io.ReadSeekCloser, *models.FileMetadata, error) {
	rootRelPath = utils.NormalizeRelPath(rootRelPath)
	if rootRelPath == "" {
		return nil, nil, errors.New("shared folder root is required")
	}
	if _, err := dm.CheckUserHasSharedFolderAccess(userId, driveId, rootRelPath); err != nil {
		return nil, nil, err
	}
	drive, err := dm.Store.GetDriveByID(driveId)
	if err != nil {
		return nil, nil, fmt.Errorf("diskmanager: get drive by id %q: %w", driveId, err)
	}
	f, md, err := dm.openFileByID(fileId)
	if err != nil {
		return nil, nil, err
	}
	// The separator suffix stops prefix-sibling folders ("share-x" vs "share")
	// from matching. rootPrefix must not be Clean()'d, which would strip it.
	rootDir := filepath.Join(dm.Home, drive.Slug, filepath.FromSlash(rootRelPath))
	rootPrefix := rootDir + string(os.PathSeparator)
	if !strings.HasPrefix(filepath.Clean(md.PathKey), rootPrefix) {
		f.Close()
		return nil, nil, errors.New("file is outside the shared folder")
	}
	return f, md, nil
}

// DownloadSharedURL is unsupported for local disk and always returns "". It
// still validates the share so authorization behavior matches
// DownloadSharedFile.
func (dm *DiskManager) DownloadSharedURL(fileId, rootRelPath, driveId, userId string, inline bool) (string, error) {
	rootRelPath = utils.NormalizeRelPath(rootRelPath)
	if rootRelPath == "" {
		return "", errors.New("shared folder root is required")
	}
	if _, err := dm.CheckUserHasSharedFolderAccess(userId, driveId, rootRelPath); err != nil {
		return "", err
	}
	return "", nil
}
