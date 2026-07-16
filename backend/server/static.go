package server

import (
	"net/http"
	"os"
	"path/filepath"
)

// StaticDirEnv overrides where the built frontend is looked up.
const StaticDirEnv = "ARCHIVUS_STATIC_DIR"

// DefaultStaticDir returns the frontend directory shipped next to the binary,
// which is the layout the release tarball unpacks to. Resolving against the
// executable rather than the working directory means the server can be started
// from anywhere.
func DefaultStaticDir() string {
	if dir := os.Getenv(StaticDirEnv); dir != "" {
		return dir
	}
	exe, err := os.Executable()
	if err != nil {
		return "static"
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Join(filepath.Dir(exe), "static")
}

// SPAHandler serves the built frontend from dir. Requests for files that exist
// are served as-is; anything else returns index.html so client-side routing can
// handle the path.
func SPAHandler(dir string) http.Handler {
	fileServer := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean before joining so "/../" cannot escape dir.
		name := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(name); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, index)
	})
}

// hasFrontend reports whether dir holds a usable build. When it does not, the
// server stays API-only rather than serving 404s that look like a broken app.
func hasFrontend(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "index.html"))
	return err == nil && !info.IsDir()
}
