package webdavfs

import (
	storage_types "archivus/internal/services/storagemanager/types"
	"io/fs"
	"time"
)

// fileInfo adapts a storage StatInfo / DirEntry into an os.FileInfo for the
// golang.org/x/net/webdav layer.
type fileInfo struct {
	name    string
	size    int64
	isDir   bool
	modTime time.Time
}

func statToFileInfo(si *storage_types.StatInfo) fileInfo {
	return fileInfo{name: si.Name, size: si.Size, isDir: si.IsDir, modTime: si.ModTime}
}

func (fi fileInfo) Name() string { return fi.name }
func (fi fileInfo) Size() int64  { return fi.size }

func (fi fileInfo) Mode() fs.FileMode {
	if fi.isDir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}

func (fi fileInfo) ModTime() time.Time { return fi.modTime }
func (fi fileInfo) IsDir() bool        { return fi.isDir }
func (fi fileInfo) Sys() any           { return nil }
