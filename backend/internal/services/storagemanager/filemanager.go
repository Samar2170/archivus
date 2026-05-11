package storagemanager

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
)

func (dm *StorageManager) UploadFile(relPath, driveId, userId string, file multipart.File, fileHeader *multipart.FileHeader) error {
	hasAccess, err := dm.checkUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("user does not have write access to this drive")
	}
	drive, err := dm.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("storagemanager: get drive by id %q: %w", driveId, err)
	}
	dirPath := filepath.Join(dm.Home, drive.Slug, relPath)
	relPath = filepath.Join(drive.Slug, relPath, fileHeader.Filename)
	absPath := filepath.Join(dm.Home, relPath)
	outFile, err := os.Create(absPath)
	if err != nil {
		return fmt.Errorf("storagemanager: create file %q: %w", absPath, err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, file); err != nil {
		return fmt.Errorf("storagemanager: save file %q: %w", absPath, err)
	}

	sizeInMb := float64(fileHeader.Size) / (1 << 20)
	_, err = dm.Store.CreateFileMetadata(fileHeader.Filename, absPath, relPath, dirPath, driveId, userId, sizeInMb)
	if err != nil {
		err = os.Remove(absPath) // cleanup uploaded file on failure
		if err != nil {
			fmt.Printf("warning: failed to clean up file after db error: %v\n", err)
		}
		return fmt.Errorf("storagemanager: create file metadata for file %q: %w", absPath, err)
	}
	return nil
}

type DirEntry struct {
	ID        uint
	Name      string
	IsDir     bool
	Extension string
	SignedUrl string
	Size      float64
	Path      string
	Thumbnail string

	NavigationPath string
}

func (dm *StorageManager) GetFiles(relPath, driveId, userId string) ([]DirEntry, error) {
	hasAccess, err := dm.checkUserHasDriveAccess(userId, driveId)
	if err != nil {
		return nil, err
	}
	if !hasAccess {
		return nil, errors.New("user does not have access to this drive")
	}
	drive, err := dm.Store.GetDriveByID(driveId)
	if err != nil {
		return nil, fmt.Errorf("storagemanager: get drive by id %q: %w", driveId, err)
	}
	files, err := os.ReadDir(filepath.Join(dm.Home, drive.Slug, relPath))
	if err != nil {
		return nil, fmt.Errorf("storagemanager: read directory %q: %w", filepath.Join(dm.Home, drive.Slug, relPath), err)
	}
	mds, err := dm.Store.ListFilesByRelPath(filepath.Join(dm.Home, drive.Slug, relPath))
	if err != nil {
		return nil, fmt.Errorf("storagemanager: list files by directory path %q: %w", filepath.Join(drive.Slug, relPath), err)
	}
	mdIdMap := make(map[string]string)
	for _, md := range mds {
		mdIdMap[md.Name] = fmt.Sprintf("%d", md.ID)
	}
	var dirEntries []DirEntry
	for i, entry := range files {
		d := DirEntry{
			ID:             uint(i),
			Name:           entry.Name(),
			IsDir:          entry.IsDir(),
			Extension:      filepath.Ext(entry.Name()),
			Path:           filepath.Join(drive.Slug, relPath, entry.Name()),
			NavigationPath: filepath.Join(relPath, entry.Name()),
		}
		dirEntries = append(dirEntries, d)
	}
	return dirEntries, nil

}
