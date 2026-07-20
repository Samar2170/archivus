// Package syncer uploads local files and directories to a drive and keeps
// tracked folders synced. It only uploads files that are new or whose content
// has changed since the last run, tracked in a local state file.
package syncer

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"

	"archivus-sync/internal/api"
	"archivus-sync/internal/config"
)

// maxUploadSize mirrors the backend's per-file limit (archivus_constants.MaxUploadSize).
// Larger files are skipped with a warning rather than failing the whole run.
const maxUploadSize = 100 << 20 // 100 MB

// Syncer coordinates change detection and uploads against a single backend.
type Syncer struct {
	cfg    *config.Config
	client *api.Client
	state  *state
	log    *log.Logger
}

// New builds a Syncer from a logged-in config.
func New(cfg *config.Config, logger *log.Logger) (*Syncer, error) {
	if !cfg.LoggedIn() {
		return nil, fmt.Errorf("not logged in: run `archivus-sync login` first")
	}
	st, err := loadState()
	if err != nil {
		return nil, err
	}
	return &Syncer{
		cfg:    cfg,
		client: api.New(cfg.ServerURL, cfg.Token),
		state:  st,
		log:    logger,
	}, nil
}

// Result summarizes a sync/upload run.
type Result struct {
	Uploaded int
	Skipped  int
	Failed   int
}

func (r *Result) add(o Result) {
	r.Uploaded += o.Uploaded
	r.Skipped += o.Skipped
	r.Failed += o.Failed
}

// UploadPath uploads a single file or a directory tree to driveID under destRel.
// When force is false, unchanged files (per the local state) are skipped.
func (s *Syncer) UploadPath(ctx context.Context, localPath, driveID, destRel string, force bool) (Result, error) {
	abs, err := filepath.Abs(localPath)
	if err != nil {
		return Result{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Result{}, fmt.Errorf("stat %q: %w", abs, err)
	}
	if info.IsDir() {
		res := s.uploadTree(ctx, abs, driveID, destRel, force)
		if err := s.state.save(); err != nil {
			return res, err
		}
		return res, nil
	}
	// Single file: it lands directly under destRel.
	res := s.uploadOne(ctx, abs, info, driveID, destRel, force)
	if err := s.state.save(); err != nil {
		return res, err
	}
	return res, nil
}

// SyncAll syncs every tracked folder once. Intended to be invoked periodically
// (external cron/systemd timer, or the built-in daemon).
func (s *Syncer) SyncAll(ctx context.Context) (Result, error) {
	if len(s.cfg.Tracked) == 0 {
		s.log.Printf("no tracked folders configured; nothing to sync")
		return Result{}, nil
	}
	var total Result
	for _, tf := range s.cfg.Tracked {
		info, err := os.Stat(tf.LocalPath)
		if err != nil || !info.IsDir() {
			s.log.Printf("skipping tracked folder %q: not an accessible directory", tf.LocalPath)
			total.Failed++
			continue
		}
		s.log.Printf("syncing %q -> drive %s /%s", tf.LocalPath, tf.DriveID, tf.DestRel)
		res := s.uploadTree(ctx, tf.LocalPath, tf.DriveID, tf.DestRel, false)
		total.add(res)
	}
	if err := s.state.save(); err != nil {
		return total, err
	}
	s.log.Printf("sync complete: %d uploaded, %d unchanged, %d failed", total.Uploaded, total.Skipped, total.Failed)
	return total, nil
}

// uploadTree walks root and uploads every regular file, preserving the directory
// structure under destRel. It does not save state; the caller does once at the end.
func (s *Syncer) uploadTree(ctx context.Context, root, driveID, destRel string, force bool) Result {
	var res Result
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			s.log.Printf("skip %q: %v", p, err)
			res.Failed++
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil // skip symlinks, sockets, devices
		}
		info, err := d.Info()
		if err != nil {
			s.log.Printf("skip %q: %v", p, err)
			res.Failed++
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			s.log.Printf("skip %q: %v", p, err)
			res.Failed++
			return nil
		}
		// Destination folder for this file = destRel + the file's subdirectory
		// within the tree (forward-slashed; drive paths are not OS paths).
		subDir := filepath.ToSlash(filepath.Dir(rel))
		folderPath := destRel
		if subDir != "." {
			folderPath = path.Join(destRel, subDir)
		}
		one := s.uploadOne(ctx, p, info, driveID, folderPath, force)
		res.add(one)
		return nil
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		s.log.Printf("walk %q: %v", root, err)
	}
	return res
}

// uploadOne uploads a single file if it is new or changed (or force is set),
// updating the in-memory state. It never returns an error: per-file failures are
// logged and counted so one bad file doesn't abort a whole sync.
func (s *Syncer) uploadOne(ctx context.Context, absPath string, info os.FileInfo, driveID, folderPath string, force bool) Result {
	if info.Size() > maxUploadSize {
		s.log.Printf("skip %q: %d bytes exceeds server limit of %d", absPath, info.Size(), maxUploadSize)
		return Result{Failed: 1}
	}

	prev, known := s.state.Files[absPath]
	fastUnchanged := known && prev.Size == info.Size() && prev.ModTime == info.ModTime().UnixNano()
	if fastUnchanged && !force {
		return Result{Skipped: 1}
	}

	// Size or mtime differs (or unknown): confirm with a content hash before
	// deciding to upload. This also collapses touch-only changes.
	sum, err := sha256File(absPath)
	if err != nil {
		s.log.Printf("skip %q: hash: %v", absPath, err)
		return Result{Failed: 1}
	}
	if known && sum == prev.Checksum && !force {
		// Content identical; refresh recorded metadata so the fast path hits next time.
		s.state.Files[absPath] = fileState{Size: info.Size(), ModTime: info.ModTime().UnixNano(), Checksum: sum}
		return Result{Skipped: 1}
	}

	if err := s.client.UploadFile(ctx, driveID, folderPath, absPath); err != nil {
		s.log.Printf("upload %q -> /%s: %v", absPath, folderPath, err)
		return Result{Failed: 1}
	}
	s.state.Files[absPath] = fileState{Size: info.Size(), ModTime: info.ModTime().UnixNano(), Checksum: sum}
	dest := folderPath
	if dest == "" {
		dest = "(root)"
	}
	s.log.Printf("uploaded %q -> %s/%s", absPath, driveID, dest)
	return Result{Uploaded: 1}
}
