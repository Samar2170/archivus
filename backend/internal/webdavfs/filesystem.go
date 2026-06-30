// Package webdavfs adapts the Archivus StorageManager to the
// golang.org/x/net/webdav FileSystem interface, scoped to a single drive and
// authenticated user. The webdav.Handler drives the WebDAV protocol (PROPFIND,
// GET, PUT, MKCOL, DELETE, MOVE, COPY, LOCK) on top of these primitives.
package webdavfs

import (
	"archivus/internal/services/storagemanager"
	"context"
	"io/fs"
	"os"
	"strings"

	"golang.org/x/net/webdav"
)

// FS implements webdav.FileSystem for one drive on behalf of one user.
type FS struct {
	sm      storagemanager.StorageManager
	driveID string
	userID  string
}

// New returns a webdav.FileSystem scoped to the given drive and user.
func New(sm storagemanager.StorageManager, driveID, userID string) *FS {
	return &FS{sm: sm, driveID: driveID, userID: userID}
}

// relPath converts a webdav path ("/a/b") to a drive-relative path ("a/b").
func relPath(name string) string {
	return strings.Trim(name, "/")
}

func (f *FS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	rel := relPath(name)
	if rel == "" {
		return os.ErrExist
	}
	return f.sm.CreateDirV2(rel, f.driveID, f.userID)
}

func (f *FS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	rel := relPath(name)

	// Write intent (PUT) → buffered write file.
	if flag&(os.O_WRONLY|os.O_RDWR) != 0 {
		return newWriteFile(f, rel)
	}

	si, err := f.sm.StatPath(rel, f.driveID, f.userID)
	if err != nil {
		return nil, err
	}
	if si.IsDir {
		return &dirFile{fs: f, relPath: rel, info: statToFileInfo(si)}, nil
	}
	rsc, rinfo, err := f.sm.ReadFile(rel, f.driveID, f.userID)
	if err != nil {
		return nil, err
	}
	return &readFile{rsc: rsc, info: statToFileInfo(rinfo)}, nil
}

func (f *FS) RemoveAll(ctx context.Context, name string) error {
	return f.sm.Remove(relPath(name), f.driveID, f.userID)
}

func (f *FS) Rename(ctx context.Context, oldName, newName string) error {
	return f.sm.Rename(relPath(oldName), relPath(newName), f.driveID, f.userID)
}

func (f *FS) Stat(ctx context.Context, name string) (fs.FileInfo, error) {
	si, err := f.sm.StatPath(relPath(name), f.driveID, f.userID)
	if err != nil {
		return nil, err
	}
	return statToFileInfo(si), nil
}
