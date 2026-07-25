package main

import (
	"archivus/internal/config"
	"archivus/internal/services/oculus"
	"archivus/internal/services/storagemanager"
	"archivus/internal/services/storagemanager/diskmanager"
	"archivus/internal/services/storagemanager/s3manager"
	"archivus/internal/services/thumbnail"
	"archivus/internal/store"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/akamensky/argparse"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

// StartScheduler builds a cron scheduler, registers the periodic jobs and runs
// them until the context is cancelled. It returns the running *cron.Cron so the
// caller can add more jobs or inspect entries; call Stop (or cancel ctx) to shut
// it down gracefully, letting in-flight jobs finish.
func StartScheduler(ctx context.Context, s *store.Store) (*cron.Cron, error) {
	c := cron.New(
		cron.WithSeconds(),
		cron.WithChain(
			cron.Recover(cron.DefaultLogger),            // don't let a panicking job kill the scheduler
			cron.SkipIfStillRunning(cron.DefaultLogger), // skip a tick if the previous run is still going
		),
	)

	// Register jobs. spec is a 6-field cron expression: sec min hour dom mon dow.
	jobs := []struct {
		name string
		spec string
		fn   func()
	}{
		{
			name: "generate-thumbnails",
			spec: "0 */1 * * * *", // every 1 hour
			fn: func() {
				log.Info().Msg("cron: generating pending thumbnails")
				runThumbnailService(ctx, s)
			},
		},
		{
			name: "purge-recycle-bin",
			spec: "0 0 3 * * *", // daily at 03:00
			fn: func() {
				log.Info().Msg("cron: purging expired recycle bin items")
				runPurgeRecycleBin(ctx, s)
			},
		},
		{
			name: "mark-images",
			spec: "0 */1 * * * *", // every 5 minutes
			fn: func() {
				log.Info().Msg("cron: marking image files")
				service := oculus.NewService(s)
				if err := service.MarkImages(); err != nil {
					log.Error().Err(err).Msg("cron: failed to mark image files")
				}
			},
		},
	}

	for _, j := range jobs {
		id, err := c.AddFunc(j.spec, j.fn)
		if err != nil {
			return nil, err
		}
		log.Info().Str("job", j.name).Str("spec", j.spec).Int("entry", int(id)).Msg("cron: registered job")
	}

	c.Start()
	log.Info().Msg("cron: scheduler started")

	// Stop the scheduler when the context is cancelled.
	go func() {
		<-ctx.Done()
		log.Info().Msg("cron: stopping scheduler")
		<-c.Stop().Done() // wait for running jobs to finish
	}()

	return c, nil
}

func runThumbnailService(ctx context.Context, s *store.Store) {
	var err error
	var s3Manager *s3manager.S3Manager
	if config.Config.S3Enabled {
		s3Manager, err = s3manager.GetS3Manager(s,
			config.S3Cfg.AccountID,
			config.S3Cfg.AccessKey,
			config.S3Cfg.SecretKey,
			config.S3Cfg.BucketName,
		)
		if err != nil {
			panic(err)
		}
	}

	thumbnailService := thumbnail.NewS3Service(s3Manager, config.Config.ThumbnailDir, s)
	if err := thumbnailService.MakeThumbnails(context.Background()); err != nil {
		log.Fatal().Err(err).Msg("failed to generate thumbnails")
	}

}

// buildStorageManager constructs the storage manager matching the configured
// backend (S3 in biz mode, local disk otherwise), mirroring cmd/archivus.
func buildStorageManager(s *store.Store) (storagemanager.StorageManager, error) {
	if config.Config.S3Enabled {
		return s3manager.GetS3Manager(s,
			config.S3Cfg.AccountID,
			config.S3Cfg.AccessKey,
			config.S3Cfg.SecretKey,
			config.S3Cfg.BucketName,
		)
	}
	return diskmanager.GetDiskManager(s, config.Config.ArchivusHome), nil
}

func runPurgeRecycleBin(ctx context.Context, s *store.Store) {
	sm, err := buildStorageManager(s)
	if err != nil {
		log.Error().Err(err).Msg("cron: failed to build storage manager for recycle bin purge")
		return
	}
	if err := sm.PurgeExpiredRecycleBin(ctx); err != nil {
		log.Error().Err(err).Msg("cron: failed to purge expired recycle bin items")
	}
}

func main() {
	if config.DEBUG {
		fmt.Println("Warning DEBUG mode is enabled, running in development mode")
	}
	var err error
	parser := argparse.NewParser("archivus-v2", "A simple file archiver")
	serverMode := parser.Selector("m", "mode", []string{"home", "biz"}, &argparse.Options{
		Required: true,
		Help:     "Server mode: 'home' for personal use, 'biz' for business use",
	})

	err = parser.Parse(os.Args)
	if err != nil {
		print(parser.Usage(err))
		return
	}

	var s3ConfigPaths []string
	s3ConfigPaths, err = config.DefaultS3Paths()
	fmt.Printf("Running in %s mode\n", *serverMode)
	if err != nil && *serverMode == "biz" {
		panic(err)
	}
	if err := config.Init(*serverMode, s3ConfigPaths); err != nil {
		panic(err)
	}
	fmt.Printf("Config initialized\n")
	fmt.Println(config.Config)
	fmt.Println("ProjectBaseDir:", config.ProjectBaseDir)
	s, err := store.GetStore(config.ProjectBaseDir)
	if err != nil {
		panic(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if _, err := StartScheduler(ctx, s); err != nil {
		log.Fatal().Err(err).Msg("failed to start scheduler")
	}

	<-ctx.Done() // block until interrupted
	log.Info().Msg("celery: shutting down")

}
