package main

import (
	"archivus/internal/config"
	"archivus/internal/services/auth"
	"archivus/internal/services/storagemanager"
	"archivus/internal/services/storagemanager/diskmanager"
	"archivus/internal/services/storagemanager/s3manager"
	"archivus/internal/store"
	"archivus/server"
	"archivus/shell"
	"fmt"
	"os"

	"github.com/akamensky/argparse"
)

func main() {
	if config.DEBUG {
		fmt.Println("Warning DEBUG mode is enabled, running in development mode")
	}
	var err error
	parser := argparse.NewParser("archivus-v2", "A simple file archiver")
	serverCmd := parser.NewCommand("server", "Run the archivus server")
	serverMode := serverCmd.Selector("m", "mode", []string{"home", "biz"}, &argparse.Options{
		Required: true,
		Help:     "Server mode: 'home' for personal use, 'biz' for business use",
	})
	setupDriveCmd := parser.NewCommand("setup-drive", "Set up the drive for the current user")

	err = parser.Parse(os.Args)
	if err != nil {
		print(parser.Usage(err))
		return
	}
	var s3ConfigPath string
	if *serverMode == "biz" {
		s3ConfigPath = shell.S3Setup()
	}
	if err := config.Init(*serverMode, s3ConfigPath); err != nil {
		panic(err)
	}

	s, err := store.GetStore(config.ProjectBaseDir)
	if err != nil {
		panic(err)
	}
	var dm storagemanager.StorageManager
	if config.Config.S3Enabled {
		dm, err = s3manager.GetS3Manager(s,
			config.S3Cfg.AccountID,
			config.S3Cfg.AccessKey,
			config.S3Cfg.SecretKey,
		)
		if err != nil {
			panic(err)
		}
	} else {
		dm = diskmanager.GetDiskManager(s, config.Config.ArchivusHome)
	}
	as := auth.AuthService{
		Store:              s,
		DirManager:         dm,
		DefaultWriteAccess: config.Config.DefaultWriteAccess,
		SecretKey:          config.Config.SecretKey,
	}
	if *serverMode == "home" {
		exists, err := as.CheckMasterUser()
		if err != nil {
			panic(err)
		}
		if !exists {
			panic("master user does not exist, please run archivus setup-drive first")
		}
	}

	switch {
	case serverCmd.Happened():
		server := server.GetServer(&as)
		if err := server.ListenAndServe(); err != nil {
			panic(err)
		}
	case setupDriveCmd.Happened():
		sh := shell.Shell{AuthService: &auth.AuthService{Store: s}}
		sh.SetupDrive()
	default:
		print(parser.Usage(nil))
	}

}
