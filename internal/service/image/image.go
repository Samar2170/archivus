package image

import (
	"archivus/config"
	"archivus/internal/db"
	"archivus/internal/models"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"archivus/internal/utils"
	"os"
	"path/filepath"

	"github.com/chai2010/webp"
	"github.com/google/uuid"
)

const (
	// cwp = compressed webp
	cwp    = "_cwp"
	cwpExt = ".webp"
)

func MarkImages() error {
	var fmds []models.FileMetadata
	err := db.StorageDB.
		Where("is_image IS NULL OR is_image = ?", false).
		Find(&fmds).Error
	if err != nil {
		return utils.HandleError("MarkImages", "Failed to get file metadata records", err)
	}
	tx := db.StorageDB.Begin()
	for _, fmd := range fmds {
		ext := filepath.Ext(fmd.FilePath)
		if utils.CheckArray([]string{".jpg", ".jpeg", ".png"}, ext) {
			fmd.IsImage = true
			err := tx.Save(&fmd).Error
			if err != nil {
				tx.Rollback()
				return utils.HandleError("MarkImages", "Failed to update file metadata record", err)
			}
		}
	}
	tx.Commit()
	return nil
}

func GetCompressedPath(finalPath string) string {
	ext := filepath.Ext(finalPath)
	cleanFpath := finalPath[:len(finalPath)-len(ext)]
	cwpFpath := cleanFpath + cwp + cwpExt
	return cwpFpath
}

func compressImage(fpath string, quality int) error {
	ext := filepath.Ext(fpath)
	var finalPath string
	if config.Config.UploadsDir == fpath[:len(config.Config.UploadsDir)] {
		finalPath = fpath
	} else {
		finalPath = filepath.Join(config.Config.UploadsDir, fpath)
	}
	file, err := os.Open(finalPath)
	if err != nil {
		return utils.HandleError("compressImage", "Failed to open image file", err)
	}
	defer file.Close()

	cwpFpath := GetCompressedPath(finalPath)
	if _, err := os.Stat(cwpFpath); !os.IsNotExist(err) {
		return nil
	}

	var img image.Image
	switch ext {
	case ".jpg", ".jpeg":
		img, err = jpeg.Decode(file)
		if err != nil {
			return utils.HandleError("compressImage", "Failed to decode JPEG image", err)
		}
	case ".png":
		img, err = png.Decode(file)
		if err != nil {
			return utils.HandleError("compressImage", "Failed to decode PNG image", err)
		}
	}
	if err != nil {
		return utils.HandleError("compressImage", "Unsupported image format", err)
	}
	out, err := os.Create(cwpFpath)
	if err != nil {
		return utils.HandleError("compressImage", "Failed to create compressed image file", err)
	}
	defer out.Close()
	err = webp.Encode(out, img, &webp.Options{
		Quality: float32(quality),
	})
	if err != nil {
		return utils.HandleError("compressImage", "Failed to encode image to WebP format", err)
	}
	return nil
}

func CompressImages(quality int) error {
	var fmds []models.FileMetadata
	var userIds []uuid.UUID
	err := db.StorageDB.Where("compress_images = ?", true).Pluck("user_id", &userIds).Error
	if err != nil {
		return utils.HandleError("CompressImages", "Failed to get user IDs with compress_images enabled", err)
	}
	err = db.StorageDB.
		Where("compressed_version_available = ? OR compressed_version_available IS NULL", false).
		Where("is_image = ?", true).
		Find(&fmds).Error
	if err != nil {
		return utils.HandleError("CompressImages", "Failed to get file metadata records", err)
	}
	for _, fmd := range fmds {
		err := compressImage(fmd.FilePath, quality)
		if err != nil {
			utils.LogError("CompressImages", fmt.Sprintf("Failed to compress image: %s", fmd.FilePath), err)
			continue
		}
		fmd.CompressedVersionAvailable = true
		db.StorageDB.Save(&fmd)
	}
	return nil
}
