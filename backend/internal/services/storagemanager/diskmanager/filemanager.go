package diskmanager

import (
	"archivus/internal/models"
	storage_types "archivus/internal/services/storagemanager/types"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
)

func (dm *DiskManager) UploadFile(relPath, driveId, userId string, file multipart.File, fileHeader *multipart.FileHeader) error {
	hasAccess, err := dm.checkUserDriveWriteAccess(userId, driveId)
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
	dirPath := filepath.Join(dm.Home, drive.Slug, relPath)
	relPath = filepath.Join(drive.Slug, relPath, fileHeader.Filename)
	absPath := filepath.Join(dm.Home, relPath)
	outFile, err := os.Create(absPath)
	if err != nil {
		return fmt.Errorf("diskmanager: create file %q: %w", absPath, err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, file); err != nil {
		return fmt.Errorf("diskmanager: save file %q: %w", absPath, err)
	}

	sizeInMb := float64(fileHeader.Size) / (1 << 20)
	_, err = dm.Store.CreateFileMetadata(fileHeader.Filename, absPath, relPath, dirPath, driveId, userId, sizeInMb)
	if err != nil {
		err = os.Remove(absPath) // cleanup uploaded file on failure
		if err != nil {
			fmt.Printf("warning: failed to clean up file after db error: %v\n", err)
		}
		return fmt.Errorf("diskmanager: create file metadata for file %q: %w", absPath, err)
	}
	return nil
}

func (dm *DiskManager) MoveFile(srcRelPath, dstRelPath, driveId, userId string) error {
	hasAccess, err := dm.checkUserDriveWriteAccess(userId, driveId)
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

	srcAbs := filepath.Join(dm.Home, drive.Slug, srcRelPath)
	dstAbs := filepath.Join(dm.Home, drive.Slug, dstRelPath)

	md, err := dm.Store.GetFileMetadataByRelPath(filepath.Join(drive.Slug, srcRelPath))
	if err != nil {
		return fmt.Errorf("diskmanager: get file metadata for %q: %w", srcRelPath, err)
	}

	if err := os.Rename(srcAbs, dstAbs); err != nil {
		return fmt.Errorf("diskmanager: move file %q to %q: %w", srcAbs, dstAbs, err)
	}

	newRelPath := filepath.Join(drive.Slug, dstRelPath)
	newDirPath := filepath.Join(dm.Home, drive.Slug, filepath.Dir(dstRelPath))

	if err := dm.Store.UpdateFileMetadataPaths(md.ID, dstAbs, newRelPath, newDirPath); err != nil {
		if rerr := os.Rename(dstAbs, srcAbs); rerr != nil {
			fmt.Printf("warning: failed to revert file move after db error: %v\n", rerr)
		}
		return fmt.Errorf("diskmanager: update file metadata after move: %w", err)
	}
	return nil
}

func (dm *DiskManager) DownloadFile(fileId string, driveId, userId string) (*os.File, *models.FileMetadata, error) {
	hasAccess, err := dm.checkUserHasDriveAccess(userId, driveId)
	if err != nil {
		return nil, nil, err
	}
	if !hasAccess {
		return nil, nil, errors.New("user does not have access to this drive")
	}

	md, err := dm.Store.GetFileMetadataByID(fileId)
	if err != nil {
		return nil, nil, fmt.Errorf("diskmanager: get file metadata by id %d: %w", fileId, err)
	}

	f, err := os.Open(md.AbsPath)
	if err != nil {
		return nil, nil, fmt.Errorf("diskmanager: open file %q: %w", md.AbsPath, err)
	}
	return f, &md, nil
}

func (dm *DiskManager) GetFiles(relPath, driveId, userId string) ([]storage_types.DirEntry, error) {
	hasAccess, err := dm.checkUserHasDriveAccess(userId, driveId)
	if err != nil {
		return nil, err
	}
	if !hasAccess {
		return nil, errors.New("user does not have access to this drive")
	}
	drive, err := dm.Store.GetDriveByID(driveId)
	if err != nil {
		return nil, fmt.Errorf("diskmanager: get drive by id %q: %w", driveId, err)
	}
	files, err := os.ReadDir(filepath.Join(dm.Home, drive.Slug, relPath))
	if err != nil {
		return nil, fmt.Errorf("diskmanager: read directory %q: %w", filepath.Join(dm.Home, drive.Slug, relPath), err)
	}
	mds, err := dm.Store.ListFilesByRelPath(filepath.Join(dm.Home, drive.Slug, relPath))
	if err != nil {
		return nil, fmt.Errorf("diskmanager: list files by directory path %q: %w", filepath.Join(drive.Slug, relPath), err)
	}
	mdIdMap := make(map[string]string)
	for _, md := range mds {
		mdIdMap[md.Name] = fmt.Sprintf("%d", md.ID)
	}
	var dirEntries []storage_types.DirEntry
	for i, entry := range files {
		d := storage_types.DirEntry{
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
