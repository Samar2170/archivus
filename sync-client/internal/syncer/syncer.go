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
	"strings"
	"time"

	"archivus-sync/internal/api"
	"archivus-sync/internal/config"
)

// maxUploadSize mirrors the backend's per-file limit (archivus_constants.MaxUploadSize).
// Larger files are skipped with a warning rather than failing the whole run.
const maxUploadSize = 2 << 30 // 2 GB

// chunkUploadThreshold is the size above which a file goes through the backend's
// resumable chunked endpoints instead of a single multipart POST. Large uploads
// are the ones a dropped connection hurts most: chunks land idempotently, so a
// retry only re-sends the pieces that did not make it.
const chunkUploadThreshold = 64 << 20 // 64 MB

// Syncer coordinates change detection and uploads against a single backend.
type Syncer struct {
	cfg    *config.Config
	client *api.Client
	state  *state
	log    *log.Logger
	// chunkThreshold is chunkUploadThreshold; overridden in tests.
	chunkThreshold int64
	// knownFolders remembers destination folders already created on the server,
	// keyed by drive and drive-relative path. See ensureFolder.
	knownFolders map[string]bool
}

// New builds a Syncer from a logged-in config.
func New(cfg *config.Config, logger *log.Logger) (*Syncer, error) {
	if !cfg.LoggedIn() {
		return nil, fmt.Errorf("not logged in: run `archivus-sync login` first")
	}
	st, err := loadState(cfg.ServerURL)
	if err != nil {
		return nil, err
	}
	return &Syncer{
		cfg:            cfg,
		client:         api.New(cfg.ServerURL, cfg.Token),
		state:          st,
		log:            logger,
		chunkThreshold: chunkUploadThreshold,
		knownFolders:   map[string]bool{},
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
		if err := s.state.save(s.cfg.ServerURL); err != nil {
			return res, err
		}
		return res, nil
	}
	// Single file: it lands directly under destRel.
	res := s.uploadOne(ctx, abs, info, driveID, destRel, force)
	if err := s.state.save(s.cfg.ServerURL); err != nil {
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
	if err := s.state.save(s.cfg.ServerURL); err != nil {
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
		// Nothing to send, so any session left over from an earlier attempt is
		// never going to be resumed.
		s.discardPending(ctx, absPath)
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
		s.discardPending(ctx, absPath)
		return Result{Skipped: 1}
	}

	// Only now that the file is definitely being sent is the destination worth
	// creating; an unchanged tree should not touch the server at all.
	if err := s.ensureFolder(ctx, driveID, folderPath); err != nil {
		s.log.Printf("skip %q: destination folder /%s: %v", absPath, folderPath, err)
		return Result{Failed: 1}
	}
	if err := s.upload(ctx, absPath, info, driveID, folderPath, sum); err != nil {
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

// ensureFolder makes folderPath exist in the drive, creating every level of it
// that the server does not already have.
//
// The backend builds its folder listing from explicit create calls, not from the
// keys files are uploaded under, so a destination that only exists locally is
// unknown to it: a chunked upload is rejected outright at init, and a plain
// upload lands at a key no listing can reach. Each level is created in turn
// because the local-disk backend records only the leaf of a create, which would
// otherwise leave the intermediate folders of a nested tree unlistable.
//
// Creates are idempotent, and what has been created is cached for the life of
// the Syncer, so the steady-state cost is one call per folder that actually
// receives a file.
func (s *Syncer) ensureFolder(ctx context.Context, driveID, folderPath string) error {
	if folderPath == "" {
		return nil // the drive root always exists
	}
	segments := strings.Split(folderPath, "/")
	for i := range segments {
		sub := strings.Join(segments[:i+1], "/")
		key := driveID + "\x00" + sub
		if s.knownFolders[key] {
			continue
		}
		if err := s.client.CreateFolder(ctx, driveID, sub); err != nil {
			return err
		}
		s.knownFolders[key] = true
	}
	return nil
}

// upload sends one file, choosing between a single multipart POST and the
// resumable chunked flow based on size. sum is the file's content hash; it ties
// a stored session to the exact bytes it was started for.
func (s *Syncer) upload(ctx context.Context, absPath string, info os.FileInfo, driveID, folderPath, sum string) error {
	if info.Size() <= s.chunkThreshold {
		// A small file that somehow has a session recorded (threshold lowered,
		// file shrunk) will never resume it; let it go.
		s.discardPending(ctx, absPath)
		return s.client.UploadFile(ctx, driveID, folderPath, absPath)
	}
	return s.uploadChunked(ctx, absPath, info, driveID, folderPath, sum)
}

// uploadChunked resumes a stored upload session for absPath when there is a
// usable one, and otherwise starts a fresh one. Either way the session is left
// recorded in the state file until the upload completes, so a run that dies
// mid-transfer resumes from where it stopped rather than re-sending the file.
func (s *Syncer) uploadChunked(ctx context.Context, absPath string, info os.FileInfo, driveID, folderPath, sum string) error {
	if p, ok := s.state.Pending[absPath]; ok {
		if p.resumable(info.Size(), sum, driveID, folderPath, time.Now()) {
			err := s.client.ResumeChunkedUpload(ctx, p.session(), absPath)
			if err == nil {
				delete(s.state.Pending, absPath)
				s.log.Printf("resumed chunked upload of %q (session %s)", absPath, p.UploadID)
				return nil
			}
			if !errors.Is(err, api.ErrSessionGone) {
				// Keep the record: whatever chunks landed are still on the
				// server and a later run can pick up from there.
				return err
			}
			s.log.Printf("chunked upload session for %q is gone; starting over", absPath)
			delete(s.state.Pending, absPath)
		} else {
			// Different bytes, a different destination, or too old to trust:
			// the staged chunks are useless, so release them server-side.
			s.discardPending(ctx, absPath)
		}
	}

	sess, err := s.client.StartChunkedUpload(ctx, driveID, folderPath, absPath)
	if err != nil {
		return err
	}
	s.state.Pending[absPath] = pendingUpload{
		UploadID:    sess.UploadID,
		ChunkSize:   sess.ChunkSize,
		TotalChunks: sess.TotalChunks,
		Size:        info.Size(),
		Checksum:    sum,
		DriveID:     driveID,
		FolderPath:  folderPath,
		StartedAt:   time.Now().Unix(),
	}
	// Persist the session before sending a single byte. The whole point is to
	// survive a crash mid-upload, and the run's own end-of-run save will not
	// happen if the process never gets there.
	if err := s.state.save(s.cfg.ServerURL); err != nil {
		// A session that can't be recorded can never be resumed, so don't leave
		// it staged on the server.
		s.discardPending(ctx, absPath)
		return err
	}

	if err := s.client.ResumeChunkedUpload(ctx, sess, absPath); err != nil {
		return err
	}
	delete(s.state.Pending, absPath)
	return nil
}

// discardPending drops any recorded session for absPath, telling the server to
// release its staged chunks. Failures are logged, not returned: a session that
// cannot be aborted is the server's disk to reclaim, not a reason to fail the
// file.
func (s *Syncer) discardPending(ctx context.Context, absPath string) {
	p, ok := s.state.Pending[absPath]
	if !ok {
		return
	}
	delete(s.state.Pending, absPath)
	if err := s.client.AbortChunkUpload(ctx, p.UploadID); err != nil {
		s.log.Printf("abort stale upload session %s for %q: %v", p.UploadID, absPath, err)
	}
}
