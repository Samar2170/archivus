package s3manager

import (
	"archivus/internal/models"
	storage_types "archivus/internal/services/storagemanager/types"
	"archivus/internal/utils"
	"errors"
	"fmt"
	"io"
	"strings"
)

// GetSharedFiles lists a folder inside a shared subtree for a user whose access
// comes from a folder share rather than drive membership. rootRelPath must
// match an existing share for the user; relPath must stay within it.
func (s *S3Manager) GetSharedFiles(rootRelPath, relPath, driveId, userId string, page, pageSize int, query storage_types.ListFilesQuery) (storage_types.PagedDirEntries, error) {
	var out storage_types.PagedDirEntries
	rootRelPath = utils.NormalizeRelPath(rootRelPath)
	relPath = utils.NormalizeRelPath(relPath)
	if !utils.PathWithinRoot(rootRelPath, relPath) {
		return out, errors.New("requested path is outside the shared folder")
	}
	if _, err := s.CheckUserHasSharedFolderAccess(userId, driveId, rootRelPath); err != nil {
		return out, err
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return out, fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	// RootPathKey is drive-relative; re-derive the slug-prefixed key prefixes
	// this backend lists by.
	trimmed := strings.Trim(relPath, "/")
	var dirPrefixes [2]string
	if trimmed == "" {
		dirPrefixes = [2]string{drive.Slug + "/", drive.Slug}
	} else {
		dirPrefixes = [2]string{drive.Slug + "/" + trimmed + "/", drive.Slug + "/" + trimmed}
	}
	return s.listAtPrefixes(relPath, drive.ID.String(), dirPrefixes, page, pageSize, query)
}

// DownloadSharedFile serves a file from inside a shared subtree. The file must
// exist, be ready, and live under the shared root.
func (s *S3Manager) DownloadSharedFile(fileId, rootRelPath, driveId, userId string) (io.ReadSeekCloser, *models.FileMetadata, error) {
	rootRelPath = utils.NormalizeRelPath(rootRelPath)
	if rootRelPath == "" {
		return nil, nil, errors.New("shared folder root is required")
	}
	if _, err := s.CheckUserHasSharedFolderAccess(userId, driveId, rootRelPath); err != nil {
		return nil, nil, err
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return nil, nil, fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	f, md, err := s.fetchFile(fileId)
	if err != nil {
		return nil, nil, err
	}
	// PathKey is slug-prefixed ("slug/dir/filename"); the separator suffix
	// stops prefix-sibling folders ("share-x" vs "share") from matching.
	rootPrefix := drive.Slug + "/" + rootRelPath + "/"
	if !strings.HasPrefix(md.PathKey, rootPrefix) {
		f.Close()
		return nil, nil, errors.New("file is outside the shared folder")
	}
	return f, md, nil
}

// DownloadSharedURL is the direct-download counterpart of DownloadSharedFile:
// it verifies the file lives inside the shared root, then presigns an R2 URL.
func (s *S3Manager) DownloadSharedURL(fileId, rootRelPath, driveId, userId string, inline bool) (string, error) {
	rootRelPath = utils.NormalizeRelPath(rootRelPath)
	if rootRelPath == "" {
		return "", errors.New("shared folder root is required")
	}
	if _, err := s.CheckUserHasSharedFolderAccess(userId, driveId, rootRelPath); err != nil {
		return "", err
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return "", fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	md, err := s.readyFileMetadata(fileId)
	if err != nil {
		return "", err
	}
	rootPrefix := drive.Slug + "/" + rootRelPath + "/"
	if !strings.HasPrefix(md.PathKey, rootPrefix) {
		return "", errors.New("file is outside the shared folder")
	}
	return s.presignForMetadata(md, inline)
}
