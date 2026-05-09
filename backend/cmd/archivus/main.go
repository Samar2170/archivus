package main

import (
	"archivus/internal/config"
	"archivus/internal/models"
	"archivus/internal/services/auth"
	"archivus/internal/store"
	"archivus/server"
	"archivus/shell"
	"os"

	"github.com/akamensky/argparse"
)

func getStore() (*store.Store, error) {
	s := &store.Store{}
	if err := s.Init(); err != nil {
		return nil, err
	}
	if err := s.Migrate(models.User{}, models.Drive{}, models.UserInvite{}); err != nil {
		return nil, err
	}
	return s, nil
}

func main() {
	if err := config.Init(); err != nil {
		panic(err)
	}

	s, err := getStore()
	if err != nil {
		panic(err)
	}
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
		as := auth.AuthService{Store: s}
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
