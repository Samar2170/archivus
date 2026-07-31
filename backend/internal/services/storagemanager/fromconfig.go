package storagemanager

import (
	"archivus/internal/config"
	"archivus/internal/services/storagemanager/diskmanager"
	"archivus/internal/services/storagemanager/s3manager"
	"archivus/internal/store"
)

// FromConfig builds the storage manager matching the configured backend: S3 in
// biz mode, local disk otherwise. config.Init must have run first.
func FromConfig(s *store.Store) (StorageManager, error) {
	if config.Config.S3Enabled {
		return s3manager.GetS3Manager(s,
			config.S3Cfg.AccountID,
			config.S3Cfg.AccessKey,
			config.S3Cfg.SecretKey,
			config.S3Cfg.BucketName,
		)
	}
	return diskmanager.GetDiskManager(s, config.Config.ArchivusHome), nil
}
