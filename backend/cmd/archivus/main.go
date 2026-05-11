package main

import (
	"archivus/internal/config"
	"archivus/internal/services/auth"
	"archivus/internal/services/storagemanager"
	"archivus/internal/store"
	"archivus/server"
	"archivus/shell"
	"os"

	"github.com/akamensky/argparse"
)

func main() {
	if err := config.Init(); err != nil {
		panic(err)
	}

	s, err := store.GetStore(config.ProjectBaseDir)
	if err != nil {
		panic(err)
	}
	dm := storagemanager.StorageManager{Home: config.Config.ArchivusHome, Store: s, UsersHome: config.UsersDir}

	parser := argparse.NewParser("archivus-v2", "A simple file archiver")
	serverCmd := parser.NewCommand("server", "Run the archivus server")
	setupDriveCmd := parser.NewCommand("setup-drive", "Set up the drive for the current user")

	err = parser.Parse(os.Args)
	if err != nil {
		print(parser.Usage(err))
		return
	}

	switch {
	case serverCmd.Happened():
		as := auth.AuthService{Store: s, DirManager: &dm,
			DefaultWriteAccess: config.Config.DefaultWriteAccess,
			SecretKey:          config.Config.SecretKey}
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
