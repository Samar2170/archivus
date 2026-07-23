package types

import (
	archivus_constants "archivus/internal/constants"
	"path/filepath"
	"strings"
	"time"
)

// RecycleEntry is a single file held in the recycle bin, as shown to the user.
type RecycleEntry struct {
	ID           string
	Name         string
	Size         float64
	ContentType  string
	OriginalPath string
	DeletedAt    time.Time
	ExpiresAt    time.Time
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

// PagedDirEntries is a single page of a directory listing plus the totals the
// client needs to render pagination controls.
type PagedDirEntries struct {
	Entries  []DirEntry `json:"entries"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
}

// FetchWindow describes how many directory and file rows to load from the two
// backing tables for a given page. See PageWindow.
type FetchWindow struct {
	DirLimit   int
	DirOffset  int
	FileLimit  int
	FileOffset int
}

// Pagination defaults and bounds for directory listings.
const (
	DefaultPageSize = 50
	MaxPageSize     = 500
)

// PageBounds normalizes a (page, pageSize) request into a SQL (limit, offset).
// Page numbers are 1-based; a page < 1 is treated as 1. pageSize is clamped to
// [1, MaxPageSize], defaulting to DefaultPageSize when <= 0.
func PageBounds(page, pageSize int) (limit, offset int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	return pageSize, (page - 1) * pageSize
}

// PageWindow maps a (limit, offset) page request onto the combined listing,
// which is ordered directories-first then files. It returns per-table limits
// and offsets so the store fetches only the rows the page actually needs.
//
// A limit <= 0 means "no pagination": every directory and file is returned.
func PageWindow(totalDirs, limit, offset int) FetchWindow {
	if limit <= 0 {
		return FetchWindow{DirLimit: -1, FileLimit: -1}
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= totalDirs {
		// The page lies entirely within the files region.
		return FetchWindow{
			DirLimit:   0,
			FileLimit:  limit,
			FileOffset: offset - totalDirs,
		}
	}
	// The page starts inside the directories region; it may spill into files.
	dirsReturned := totalDirs - offset
	if dirsReturned > limit {
		dirsReturned = limit
	}
	return FetchWindow{
		DirLimit:   limit,
		DirOffset:  offset,
		FileLimit:  limit - dirsReturned,
		FileOffset: 0,
	}
}

// ThumbnailURL maps a stored (absolute) thumbnail path under thumbnailDir to the
// URL path served by the static thumbnail file server. Returns "" when there is
// no thumbnail or the path escapes thumbnailDir.
func ThumbnailURL(thumbnailPath, thumbnailDir string) string {
	if thumbnailPath == "" {
		return ""
	}
	rel, err := filepath.Rel(thumbnailDir, thumbnailPath)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	return archivus_constants.ThumbnailRoutePrefix + filepath.ToSlash(rel)
}
