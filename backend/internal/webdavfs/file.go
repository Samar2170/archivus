package webdavfs

import (
	"errors"
	"io"
	"io/fs"
	"mime"
	"os"
	"path"
)

// readFile serves a regular file for GET. It wraps the ReadSeekCloser returned
// by the storage manager.
type readFile struct {
	rsc  io.ReadSeekCloser
	info fileInfo
}

func (f *readFile) Read(p []byte) (int, error)               { return f.rsc.Read(p) }
func (f *readFile) Seek(off int64, whence int) (int64, error) { return f.rsc.Seek(off, whence) }
func (f *readFile) Close() error                             { return f.rsc.Close() }
func (f *readFile) Write([]byte) (int, error)                { return 0, os.ErrPermission }
func (f *readFile) Readdir(int) ([]fs.FileInfo, error)       { return nil, errors.New("not a directory") }
func (f *readFile) Stat() (fs.FileInfo, error)               { return f.info, nil }

// dirFile represents a directory. Reads/seeks are not meaningful; Readdir lists
// children via the storage manager.
type dirFile struct {
	fs      *FS
	relPath string
	info    fileInfo
	offset  int
}

func (d *dirFile) Read([]byte) (int, error)                { return 0, os.ErrInvalid }
func (d *dirFile) Seek(int64, int) (int64, error)          { return 0, os.ErrInvalid }
func (d *dirFile) Write([]byte) (int, error)               { return 0, os.ErrPermission }
func (d *dirFile) Close() error                            { return nil }
func (d *dirFile) Stat() (fs.FileInfo, error)              { return d.info, nil }

func (d *dirFile) Readdir(count int) ([]fs.FileInfo, error) {
	entries, err := d.fs.sm.GetFilesV2(d.relPath, d.fs.driveID, d.fs.userID)
	if err != nil {
		return nil, err
	}
	infos := make([]fs.FileInfo, 0, len(entries))
	for _, e := range entries {
		infos = append(infos, fileInfo{
			name:  e.Name,
			size:  int64(e.Size * (1 << 20)), // DirEntry size is in MB; per-file Stat is exact
			isDir: e.IsDir,
		})
	}
	if count <= 0 {
		d.offset = len(infos)
		return infos, nil
	}
	if d.offset >= len(infos) {
		return nil, io.EOF
	}
	end := d.offset + count
	if end > len(infos) {
		end = len(infos)
	}
	page := infos[d.offset:end]
	d.offset = end
	return page, nil
}

// writeFile buffers a PUT to a temp file and flushes it through the storage
// manager on Close.
type writeFile struct {
	fs      *FS
	relPath string
	tmp     *os.File
	flushed bool
}

func newWriteFile(f *FS, relPath string) (*writeFile, error) {
	tmp, err := os.CreateTemp("", "archivus-webdav-put-*")
	if err != nil {
		return nil, err
	}
	return &writeFile{fs: f, relPath: relPath, tmp: tmp}, nil
}

func (w *writeFile) Write(p []byte) (int, error)               { return w.tmp.Write(p) }
func (w *writeFile) Read(p []byte) (int, error)               { return w.tmp.Read(p) }
func (w *writeFile) Seek(off int64, whence int) (int64, error) { return w.tmp.Seek(off, whence) }
func (w *writeFile) Readdir(int) ([]fs.FileInfo, error)        { return nil, errors.New("not a directory") }

func (w *writeFile) Stat() (fs.FileInfo, error) {
	info, err := w.tmp.Stat()
	if err != nil {
		return nil, err
	}
	return fileInfo{name: path.Base(w.relPath), size: info.Size(), modTime: info.ModTime()}, nil
}

func (w *writeFile) Close() error {
	defer func() {
		name := w.tmp.Name()
		w.tmp.Close()
		os.Remove(name)
	}()
	if w.flushed {
		return nil
	}
	w.flushed = true

	info, err := w.tmp.Stat()
	if err != nil {
		return err
	}
	if _, err := w.tmp.Seek(0, io.SeekStart); err != nil {
		return err
	}
	contentType := mime.TypeByExtension(path.Ext(w.relPath))
	return w.fs.sm.WriteFileStream(w.relPath, w.fs.driveID, w.fs.userID, w.tmp, info.Size(), contentType)
}
