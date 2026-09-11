package shared

import (
	"archivus/internal/models"
	"archivus/internal/services/storagemanager"
	storage_types "archivus/internal/services/storagemanager/types"
	"archivus/internal/store"
	"archivus/internal/utils"
	"errors"
	"fmt"
	"os"
	"time"
)

// Service implements the /storage/shared/* surface: read-only access to
// folders other users shared with you, plus share administration for drive
// owners/managers/admins. Drive membership and drive APIs are untouched.
type Service struct {
	Store   *store.Store
	Storage storagemanager.StorageManager
}

// SharedRoot is one folder shared with the current user, as listed by
// GET /storage/shared/roots.
type SharedRoot struct {
	ShareID     string             `json:"shareId"`
	DriveID     string             `json:"driveId"`
	DriveName   string             `json:"driveName"`
	DriveSlug   string             `json:"driveSlug"`
	RootPath    string             `json:"rootPath"`
	AccessLevel models.AccessLevel `json:"accessLevel"`
	GrantedBy   string             `json:"grantedBy"`
	GrantedAt   time.Time          `json:"grantedAt"`
}

// SharedFolderUser is one user who has a share on a folder, as listed by
// POST /storage/shared/list-users.
type SharedFolderUser struct {
	UserID      string             `json:"userId"`
	Username    string             `json:"username"`
	Email       string             `json:"email"`
	AccessLevel models.AccessLevel `json:"accessLevel"`
	GrantedBy   string             `json:"grantedBy"`
	GrantedAt   time.Time          `json:"grantedAt"`
}

// Roots lists every folder shared with the user across all drives.
func (s *Service) Roots(userID string) ([]SharedRoot, error) {
	shares, err := s.Store.ListSharedFoldersForUser(userID)
	if err != nil {
		return nil, fmt.Errorf("list shared folders: %w", err)
	}
	roots := make([]SharedRoot, 0, len(shares))
	for _, share := range shares {
		roots = append(roots, SharedRoot{
			ShareID:     share.ID.String(),
			DriveID:     share.DriveID.String(),
			DriveName:   share.Drive.Name,
			DriveSlug:   share.Drive.Slug,
			RootPath:    share.RootPathKey,
			AccessLevel: share.AccessLevel,
			GrantedBy:   share.GrantedBy.Username,
			GrantedAt:   share.CreatedAt,
		})
	}
	return roots, nil
}

// resolveShare finds the share that authorizes browsing rootPath (and path,
// when given) in a drive. The client-supplied rootPath wins when it matches an
// existing share; otherwise the deepest ancestor share of path is used, which
// keeps navigation working across nested shares.
func (s *Service) resolveShare(userID, driveID, rootPath, path string) (models.SharedInfo, models.Drive, error) {
	drive, err := s.Store.GetDriveByID(driveID)
	if err != nil {
		return models.SharedInfo{}, models.Drive{}, fmt.Errorf("invalid drive: %w", err)
	}
	shares, err := s.Store.ListSharedFoldersForDrive(driveID, userID)
	if err != nil {
		return models.SharedInfo{}, models.Drive{}, fmt.Errorf("list shares: %w", err)
	}
	if len(shares) == 0 {
		return models.SharedInfo{}, models.Drive{}, errors.New("no access to this shared folder")
	}

	var match *models.SharedInfo
	if rootPath != "" {
		for i := range shares {
			if shares[i].RootPathKey == rootPath && utils.PathWithinRoot(rootPath, path) {
				match = &shares[i]
				break
			}
		}
	}
	if match == nil {
		// Deepest ancestor share wins.
		for i := range shares {
			if !utils.PathWithinRoot(shares[i].RootPathKey, path) {
				continue
			}
			if match == nil || len(shares[i].RootPathKey) > len(match.RootPathKey) {
				match = &shares[i]
			}
		}
	}
	if match == nil {
		return models.SharedInfo{}, models.Drive{}, errors.New("requested path is outside the shared folder")
	}
	return *match, drive, nil
}

// List pages a folder inside a shared subtree. path is drive-relative and must
// be the share root or inside it.
func (s *Service) List(userID, driveID, rootPath, path string, page, pageSize int, query storage_types.ListFilesQuery) (storage_types.PagedDirEntries, error) {
	rootPath = utils.NormalizeRelPath(rootPath)
	path = utils.NormalizeRelPath(path)
	share, drive, err := s.resolveShare(userID, driveID, rootPath, path)
	if err != nil {
		return storage_types.PagedDirEntries{}, err
	}
	return s.Storage.GetSharedFiles(share.RootPathKey, path, drive.ID.String(), userID, page, pageSize, query)
}

// Download serves a file from a shared subtree. The share root must be given
// explicitly; the storage layer verifies the file lives inside it.
func (s *Service) Download(userID, driveID, rootPath, fileID string) (*os.File, *models.FileMetadata, error) {
	rootPath = utils.NormalizeRelPath(rootPath)
	if rootPath == "" {
		return nil, nil, errors.New("rootPath is required")
	}
	if fileID == "" {
		return nil, nil, errors.New("fileId is required")
	}
	share, drive, err := s.resolveShare(userID, driveID, rootPath, rootPath)
	if err != nil {
		return nil, nil, err
	}
	return s.Storage.DownloadSharedFile(fileID, share.RootPathKey, drive.ID.String(), userID)
}

// canManage reports whether userID may grant/revoke shares on a drive:
// admins, the drive owner and drive managers.
func (s *Service) canManage(userID string, drive models.Drive) (bool, error) {
	user, err := s.Store.GetUserByID(userID)
	if err != nil {
		return false, fmt.Errorf("user not found: %w", err)
	}
	if user.IsAdmin || drive.OwnerID == user.ID {
		return true, nil
	}
	inDrive, accessLevel, err := s.Store.CheckIfUserInDrive(userID, drive.ID.String())
	if err != nil {
		return false, fmt.Errorf("check drive membership: %w", err)
	}
	return inDrive && models.CompareAccessLevels(accessLevel, models.AccessLevelManager), nil
}

// Grant shares a folder with an existing Archivus user. Re-granting updates
// the access level. Only read and write are valid share levels.
func (s *Service) Grant(granterID, driveID, rootPath, userID, username string, access models.AccessLevel) (SharedFolderUser, error) {
	if access != models.AccessLevelRead && access != models.AccessLevelWrite {
		return SharedFolderUser{}, fmt.Errorf("invalid access level %q: folder shares are read or write", access)
	}
	drive, err := s.Store.GetDriveByID(driveID)
	if err != nil {
		return SharedFolderUser{}, fmt.Errorf("invalid drive: %w", err)
	}
	allowed, err := s.canManage(granterID, drive)
	if err != nil {
		return SharedFolderUser{}, err
	}
	if !allowed {
		return SharedFolderUser{}, errors.New("only drive owners, managers and admins can share folders")
	}
	target, err := s.Store.ResolveUserByUsernameOrId(username, userID)
	if err != nil {
		return SharedFolderUser{}, fmt.Errorf("target user not found: %w", err)
	}
	if target.ID.String() == granterID {
		return SharedFolderUser{}, errors.New("cannot share a folder with yourself")
	}
	rootPath = utils.NormalizeRelPath(rootPath)
	if rootPath == "" {
		return SharedFolderUser{}, errors.New("rootPath is required (whole drives are shared through drive membership, not folder shares)")
	}
	// The folder must exist in this drive; DirExists requires write access,
	// which every manager-level user has.
	exists, err := s.Storage.DirExists(rootPath, drive.ID.String(), granterID)
	if err != nil {
		return SharedFolderUser{}, fmt.Errorf("look up folder %q: %w", rootPath, err)
	}
	if !exists {
		return SharedFolderUser{}, fmt.Errorf("folder %q does not exist in this drive", rootPath)
	}
	share, err := s.Store.GrantSharedFolder(drive.ID.String(), rootPath, target.ID.String(), granterID, access)
	if err != nil {
		return SharedFolderUser{}, fmt.Errorf("grant share: %w", err)
	}
	granter, err := s.Store.GetUserByID(granterID)
	if err != nil {
		return SharedFolderUser{}, fmt.Errorf("granter not found: %w", err)
	}
	return SharedFolderUser{
		UserID:      target.ID.String(),
		Username:    target.Username,
		Email:       target.Email,
		AccessLevel: share.AccessLevel,
		GrantedBy:   granter.Username,
		GrantedAt:   share.CreatedAt,
	}, nil
}

// Revoke removes a user's share on a folder. Access is lost immediately.
func (s *Service) Revoke(granterID, driveID, rootPath, userID, username string) error {
	drive, err := s.Store.GetDriveByID(driveID)
	if err != nil {
		return fmt.Errorf("invalid drive: %w", err)
	}
	allowed, err := s.canManage(granterID, drive)
	if err != nil {
		return err
	}
	if !allowed {
		return errors.New("only drive owners, managers and admins can manage folder shares")
	}
	target, err := s.Store.ResolveUserByUsernameOrId(username, userID)
	if err != nil {
		return fmt.Errorf("target user not found: %w", err)
	}
	rootPath = utils.NormalizeRelPath(rootPath)
	if rootPath == "" {
		return errors.New("rootPath is required")
	}
	return s.Store.RevokeSharedFolderByScope(drive.ID.String(), rootPath, target.ID.String())
}

// Users lists everyone who currently has a share on a folder.
func (s *Service) Users(userID, driveID, rootPath string) ([]SharedFolderUser, error) {
	drive, err := s.Store.GetDriveByID(driveID)
	if err != nil {
		return nil, fmt.Errorf("invalid drive: %w", err)
	}
	allowed, err := s.canManage(userID, drive)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, errors.New("only drive owners, managers and admins can view folder shares")
	}
	rootPath = utils.NormalizeRelPath(rootPath)
	if rootPath == "" {
		return nil, errors.New("rootPath is required")
	}
	shares, err := s.Store.ListSharedFoldersForFolder(drive.ID.String(), rootPath)
	if err != nil {
		return nil, fmt.Errorf("list folder shares: %w", err)
	}
	users := make([]SharedFolderUser, 0, len(shares))
	for _, share := range shares {
		users = append(users, SharedFolderUser{
			UserID:      share.SharedWithUserID.String(),
			Username:    share.SharedWith.Username,
			Email:       share.SharedWith.Email,
			AccessLevel: share.AccessLevel,
			GrantedBy:   share.GrantedBy.Username,
			GrantedAt:   share.CreatedAt,
		})
	}
	return users, nil
}
