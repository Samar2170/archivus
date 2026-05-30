package s3manager

import (
	"archivus/internal/store"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

type S3Manager struct {
	Client *Client
	Store  *store.Store
}

func GetS3Manager(s *store.Store, accountID, accessKey, secretKey, bucketName string) (*S3Manager, error) {
	client, err := New(accountID, accessKey, secretKey, bucketName)
	if err != nil {
		return nil, err
	}
	return &S3Manager{Client: client, Store: s}, nil
}

func (s *S3Manager) CreateDriveDir(driveName string) (string, error) {
	return driveName, nil
}

// DeleteDriveDir deletes a drive bucket by name. Only useful as error-path cleanup
// immediately after CreateDriveDir, since the slug is re-derived and won't match
// an existing bucket created in a previous call.
func (s *S3Manager) DeleteDriveDir(driveName string) error {
	return nil
}

func (s *S3Manager) checkUserDriveWriteAccess(userID, driveID string) (bool, error) {
	user, err := s.Store.GetUserByID(userID)
	if err != nil {
		return false, fmt.Errorf("s3manager: get user %q: %w", userID, err)
	}
	if !user.WriteAccess {
		return false, nil
	}
	return s.Store.CheckIfUserInDrive(userID, driveID)
}

func (s *S3Manager) checkUserHasDriveAccess(userID, driveID string) (bool, error) {
	return s.Store.CheckIfUserInDrive(userID, driveID)
}

func (s *S3Manager) CreateDir(subFolder, driveId, userId string) error {
	if subFolder == "" {
		return errors.New("subFolder cannot be empty")
	}
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
	// trailing slash is the S3 convention for virtual directories
	key := strings.Trim(subFolder, "/") + "/"
	if err := s.Client.CreateDirectory(context.Background(), drive.Slug, key); err != nil {
		return fmt.Errorf("s3manager: create directory %q in bucket %q: %w", key, drive.Slug, err)
	}
	relPath := filepath.Join(drive.Slug, subFolder)
	absPath := fmt.Sprintf("s3://%s/%s", drive.Slug, key)
	parts := strings.Split(strings.Trim(subFolder, "/"), "/")
	name := parts[len(parts)-1]
	_, err = s.Store.CreateDirectoryMetadata(name, absPath, relPath, drive.ID.String())
	if err != nil {
		_ = s.Client.DeleteObject(context.Background(), drive.Slug, key)
		return fmt.Errorf("s3manager: save directory metadata for %q: %w", key, err)
	}
	return nil
}

func (s *S3Manager) DeleteDir(relPath, driveId, userId string) error {
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
	prefix := s3Key(relPath, drive.Slug)
	ctx := context.Background()
	keys, err := s.Client.ListObjects(ctx, drive.Slug, prefix)
	if err != nil {
		return fmt.Errorf("s3manager: list prefix %q: %w", prefix, err)
	}
	if len(keys) > 0 {
		if err := s.Client.DeleteObjects(ctx, drive.Slug, keys); err != nil {
			return fmt.Errorf("s3manager: delete prefix %q: %w", prefix, err)
		}
	}
	return s.Store.DeleteDirectoryMetadataByRelPath(relPath)
}

// s3Key converts a DB relPath (drive.Slug/subpath) to an S3 object key (subpath).
func s3Key(relPath, driveSlug string) string {
	return strings.TrimPrefix(relPath, driveSlug+"/")
}
