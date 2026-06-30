package types

import "time"

// StatInfo describes a single file or directory at a given path, independent
// of the storage backend. Used by the WebDAV layer to answer Stat requests.
type StatInfo struct {
	Name    string
	IsDir   bool
	Size    int64
	ModTime time.Time
}

type DirEntry struct {
	ID        string
	Name      string
	IsDir     bool
	Extension string
	SignedUrl string
	Size      float64
	Path      string
	Thumbnail string

	NavigationPath string
}
