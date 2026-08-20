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

	// CreatedAt is when the file or directory was created. Empty for entries
	// built outside the metadata store (e.g. legacy listings).
	CreatedAt time.Time
	// ContentType is the file's MIME type. Empty for directories.
	ContentType string

	// UploadStatus is the backing file's persistence state ("ready", "pending",
	// "uploading", "failed"). Empty for directories.
	UploadStatus string

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

// Sort keys accepted by directory listings. Anything else falls back to name.
const (
	SortByName      = "name"
	SortBySize      = "size"
	SortByCreatedAt = "created_at"

	SortOrderAsc  = "asc"
	SortOrderDesc = "desc"
)

// Content-type filter categories. Media categories match by prefix (every
// subtype); document categories match an explicit set of MIME types; "others"
// matches everything not covered by any other category.
const (
	CategoryImages       = "images"
	CategoryVideos       = "videos"
	CategoryAudio        = "audio"
	CategorySpreadsheets = "spreadsheets"
	CategoryDocs         = "docs"
	CategoryPDFs         = "pdfs"
	CategoryCode         = "code"
	CategoryOthers       = "others"
)

// categoryContentTypes maps a category to the content types it covers. An entry
// ending in "/" is a prefix that matches every subtype (e.g. "image/"). The
// "others" category is deliberately absent: it is the complement of everything
// listed here.
var categoryContentTypes = map[string][]string{
	CategoryImages: {"image/"},
	CategoryVideos: {"video/"},
	CategoryAudio:  {"audio/"},
	CategorySpreadsheets: {
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.oasis.opendocument.spreadsheet",
		"application/vnd.apple.numbers",
		"text/csv",
		"application/csv",
		"text/tab-separated-values",
	},
	CategoryDocs: {
		"application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.oasis.opendocument.text",
		"application/rtf",
		"application/vnd.apple.pages",
		"application/vnd.google-apps.document",
		"text/plain",
		"text/markdown",
	},
	CategoryPDFs: {"application/pdf"},
	CategoryCode: {
		"text/x-python", "application/x-python", "text/x-python-script",
		"text/javascript", "application/javascript", "application/x-javascript", "text/ecmascript",
		"application/x-typescript", "text/typescript",
		"text/html", "application/xhtml+xml",
		"text/css",
		"text/x-c", "text/x-csrc", "text/x-c++src", "text/x-chdr",
		"text/x-java", "text/x-java-source",
		"application/json", "text/json",
		"text/x-go",
		"application/x-sh", "text/x-shellscript",
		"application/x-ruby", "text/x-ruby",
		"text/x-php",
		"application/xml", "text/xml",
		"text/x-yaml", "application/x-yaml", "application/yaml",
		"text/x-sql", "application/sql",
		"text/x-rust",
		"text/x-swift",
		"application/x-toml", "text/x-toml",
		"application/x-lua", "text/x-lua",
	},
}

// CategoryContentTypes returns the content-type patterns for category. A
// pattern ending in "/" matches by prefix (every subtype); all others match
// exactly. It returns nil for "" and for unknown categories (including
// CategoryOthers, whose membership the caller computes as the complement).
func CategoryContentTypes(category string) []string {
	return categoryContentTypes[category]
}

// Categories returns every named category except "others", in a stable order.
func Categories() []string {
	return []string{
		CategoryImages,
		CategoryVideos,
		CategoryAudio,
		CategorySpreadsheets,
		CategoryDocs,
		CategoryPDFs,
		CategoryCode,
	}
}

// ValidCategory reports whether category is one of the known filter categories.
func ValidCategory(category string) bool {
	switch category {
	case CategoryImages, CategoryVideos, CategoryAudio, CategorySpreadsheets,
		CategoryDocs, CategoryPDFs, CategoryCode, CategoryOthers:
		return true
	}
	return false
}

// ListOptions controls how a directory listing is ordered and filtered. The
// zero value is the default listing: files and directories ordered by name
// ascending, with no content-type filter applied.
type ListOptions struct {
	// SortBy is one of SortByName, SortBySize, or SortByCreatedAt. Unknown or
	// empty values are treated as SortByName.
	SortBy string
	// SortOrder is SortOrderAsc or SortOrderDesc. Unknown or empty values are
	// treated as ascending.
	SortOrder string
	// Category groups the content-type filter into a named category (images,
	// videos, ...). When set it takes precedence over ContentType.
	Category string
	// ContentType, when non-empty (and Category empty), restricts the returned
	// files to those whose content type matches exactly. Directories are never
	// filtered out.
	ContentType string
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
