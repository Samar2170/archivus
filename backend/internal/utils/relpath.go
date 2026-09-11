package utils

import (
	"path"
	"strings"
)

// NormalizeRelPath cleans a drive-relative folder path: slash separators, no
// leading or trailing slash, no "." or ".." segments. The empty string means
// the drive root. Examples: "/a//b/" -> "a/b", "a/../b" -> "b", "." -> "".
func NormalizeRelPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = path.Clean("/" + p)
	p = strings.TrimPrefix(p, "/")
	if p == "." {
		return ""
	}
	return p
}

// PathWithinRoot reports whether relPath is root itself or lies inside root's
// subtree. An empty root means the drive root, which contains everything.
// Both arguments are expected normalized (see NormalizeRelPath).
func PathWithinRoot(root, relPath string) bool {
	if root == "" {
		return true
	}
	return relPath == root || strings.HasPrefix(relPath, root+"/")
}
