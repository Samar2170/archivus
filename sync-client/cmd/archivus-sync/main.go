// Command archivus-sync is a standalone desktop client for an archivus server.
// It uploads files and directories, and keeps local folders synced up into a
// drive, re-uploading only files whose content has changed.
//
// Usage:
//
//	archivus-sync login   -server URL -username NAME
//	archivus-sync upload   PATH -drive DRIVE_ID [-dest FOLDER] [-force]
//	archivus-sync track add PATH -drive DRIVE_ID [-dest FOLDER]
//	archivus-sync track list
//	archivus-sync track remove PATH
//	archivus-sync sync                  # sync all tracked folders once (for cron)
//	archivus-sync daemon [-interval 15m]
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"archivus-sync/internal/api"
	"archivus-sync/internal/config"
	"archivus-sync/internal/syncer"
)

func main() {
	log.SetFlags(log.LstdFlags)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "login":
		err = cmdLogin(args)
	case "upload":
		err = cmdUpload(args)
	case "track":
		err = cmdTrack(args)
	case "sync":
		err = cmdSync(args)
	case "daemon":
		err = cmdDaemon(args)
	case "help", "-h", "--help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `archivus-sync - desktop sync client for archivus

Commands:
  login    -server URL -username NAME       Authenticate and store the token
  upload   PATH -drive ID [-dest F] [-force] Upload a file or directory once
  track    add PATH -drive ID [-dest F]      Track a folder for syncing
  track    list                             List tracked folders
  track    remove PATH                       Stop tracking a folder
  sync                                      Sync all tracked folders once (cron)
  daemon   [-interval 15m]                   Sync tracked folders on a loop

Config and state live under ~/.archivus-sync (override with ARCHIVUS_SYNC_HOME).
`)
}

// cmdLogin authenticates against the server and persists the token.
func cmdLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	server := fs.String("server", "", "server base URL, e.g. http://localhost:8080")
	username := fs.String("username", "", "username")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	serverURL := firstNonEmpty(*server, cfg.ServerURL)
	if serverURL == "" {
		serverURL = prompt("Server URL (e.g. http://localhost:8080): ")
	}
	user := firstNonEmpty(*username, cfg.Username)
	if user == "" {
		user = prompt("Username: ")
	}
	// Password and PIN come from env when set (useful for scripted/service setup),
	// otherwise from an interactive prompt.
	password := firstNonEmpty(os.Getenv("ARCHIVUS_SYNC_PASSWORD"), "")
	if password == "" {
		password = prompt("Password: ")
	}
	pin := firstNonEmpty(os.Getenv("ARCHIVUS_SYNC_PIN"), "")
	if pin == "" {
		pin = prompt("PIN: ")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	token, err := api.Login(ctx, serverURL, user, password, pin)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	cfg.ServerURL = strings.TrimRight(serverURL, "/")
	cfg.Username = user
	cfg.Token = token
	if err := cfg.Save(); err != nil {
		return err
	}
	path, _ := config.Path()
	fmt.Printf("Logged in as %s. Credentials saved to %s\n", user, path)
	return nil
}

func cmdUpload(args []string) error {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)
	drive := fs.String("drive", "", "destination drive id (required)")
	dest := fs.String("dest", "", "destination folder within the drive")
	force := fs.Bool("force", false, "upload even if content is unchanged")
	if err := fs.Parse(reorderFlagsFirst(args, map[string]bool{"force": true})); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: archivus-sync upload PATH -drive ID [-dest FOLDER] [-force]")
	}
	if *drive == "" {
		return fmt.Errorf("-drive is required")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	s, err := syncer.New(cfg, log.Default())
	if err != nil {
		return err
	}

	ctx, stop := signalContext()
	defer stop()
	res, err := s.UploadPath(ctx, fs.Arg(0), *drive, cleanDest(*dest), *force)
	fmt.Printf("uploaded=%d unchanged=%d failed=%d\n", res.Uploaded, res.Skipped, res.Failed)
	if err != nil {
		return err
	}
	if res.Failed > 0 {
		return fmt.Errorf("%d file(s) failed to upload", res.Failed)
	}
	return nil
}

func cmdTrack(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: archivus-sync track <add|list|remove> ...")
	}
	sub := args[0]
	rest := args[1:]

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	switch sub {
	case "add":
		fs := flag.NewFlagSet("track add", flag.ExitOnError)
		drive := fs.String("drive", "", "destination drive id (required)")
		dest := fs.String("dest", "", "destination folder within the drive")
		if err := fs.Parse(reorderFlagsFirst(rest, nil)); err != nil {
			return err
		}
		if fs.NArg() < 1 {
			return fmt.Errorf("usage: archivus-sync track add PATH -drive ID [-dest FOLDER]")
		}
		if *drive == "" {
			return fmt.Errorf("-drive is required")
		}
		abs, err := filepath.Abs(fs.Arg(0))
		if err != nil {
			return err
		}
		info, err := os.Stat(abs)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("%q is not an accessible directory", abs)
		}
		cfg.AddTracked(config.TrackedFolder{LocalPath: abs, DriveID: *drive, DestRel: cleanDest(*dest)})
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("tracking %s -> drive %s /%s\n", abs, *drive, cleanDest(*dest))
		return nil

	case "list":
		if len(cfg.Tracked) == 0 {
			fmt.Println("no tracked folders")
			return nil
		}
		for _, tf := range cfg.Tracked {
			fmt.Printf("%s -> drive %s /%s\n", tf.LocalPath, tf.DriveID, tf.DestRel)
		}
		return nil

	case "remove":
		if len(rest) < 1 {
			return fmt.Errorf("usage: archivus-sync track remove PATH")
		}
		abs, err := filepath.Abs(rest[0])
		if err != nil {
			return err
		}
		if !cfg.RemoveTracked(abs) {
			return fmt.Errorf("%q is not tracked", abs)
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("stopped tracking %s\n", abs)
		return nil

	default:
		return fmt.Errorf("unknown track subcommand %q (want add|list|remove)", sub)
	}
}

func cmdSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	s, err := syncer.New(cfg, log.Default())
	if err != nil {
		return err
	}
	ctx, stop := signalContext()
	defer stop()
	res, err := s.SyncAll(ctx)
	if err != nil {
		return err
	}
	if res.Failed > 0 {
		return fmt.Errorf("%d file(s) failed to sync", res.Failed)
	}
	return nil
}

func cmdDaemon(args []string) error {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	interval := fs.Duration("interval", 15*time.Minute, "how often to sync tracked folders")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *interval < time.Minute {
		return fmt.Errorf("-interval must be at least 1m")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	s, err := syncer.New(cfg, log.Default())
	if err != nil {
		return err
	}

	ctx, stop := signalContext()
	defer stop()

	log.Printf("daemon started; syncing every %s (Ctrl-C to stop)", *interval)
	// Run once immediately, then on each tick.
	if _, err := s.SyncAll(ctx); err != nil && ctx.Err() == nil {
		log.Printf("sync run failed: %v", err)
	}
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("daemon stopping")
			return nil
		case <-ticker.C:
			if _, err := s.SyncAll(ctx); err != nil && ctx.Err() == nil {
				log.Printf("sync run failed: %v", err)
			}
		}
	}
}

// signalContext returns a context cancelled on SIGINT/SIGTERM.
func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// cleanDest normalizes a drive-relative destination folder to forward slashes
// with no leading/trailing slashes ("" means the drive root).
func cleanDest(dest string) string {
	dest = strings.ReplaceAll(dest, "\\", "/")
	dest = strings.Trim(dest, "/")
	return dest
}

// reorderFlagsFirst moves flag tokens ahead of positional args so the stdlib
// flag package (which stops at the first non-flag arg) accepts flags placed
// after a path, e.g. `upload PATH -drive ID`. boolFlags names flags that take no
// value, so the following token isn't mistaken for their value.
func reorderFlagsFirst(args []string, boolFlags map[string]bool) []string {
	var flags, positionals []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if len(a) > 1 && strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			name := strings.TrimLeft(a, "-")
			if strings.ContainsRune(name, '=') || boolFlags[name] {
				continue // value is inline (-flag=v) or the flag is boolean
			}
			if i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		positionals = append(positionals, a)
	}
	return append(flags, positionals...)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func prompt(label string) string {
	fmt.Print(label)
	sc := bufio.NewScanner(os.Stdin)
	if sc.Scan() {
		return strings.TrimSpace(sc.Text())
	}
	return ""
}
