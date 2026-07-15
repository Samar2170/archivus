package webdavfs

import (
	storage_types "archivus/internal/services/storagemanager/types"
	"context"
	"io/fs"
	"mime"
	"path/filepath"
	"time"

	"golang.org/x/net/webdav"
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

// ContentType satisfies webdav's ContentTyper interface. Without it, PROPFIND
// opens every file just to sniff a MIME type — which on the S3 backend means
// downloading each object in full. Resolving from the extension avoids that.
func (fi fileInfo) ContentType(ctx context.Context) (string, error) {
	if fi.isDir {
		return "", webdav.ErrNotImplemented
	}
	if ct := mime.TypeByExtension(filepath.Ext(fi.name)); ct != "" {
		return ct, nil
	}
	return "application/octet-stream", nil
}
