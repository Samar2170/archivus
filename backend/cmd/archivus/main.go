package main

import (
	"archivus/internal/config"
	"archivus/internal/services/auth"
	"archivus/internal/store"
	"archivus/shell"
)

func main() {
	if err := config.Init(); err != nil {
		panic(err)
	}

	s := store.Store{}
	if err := s.Init(); err != nil {
		panic(err)
	}

	sh := shell.Shell{AuthService: &auth.AuthService{Store: &s}}
	sh.SetupDrive()
}
