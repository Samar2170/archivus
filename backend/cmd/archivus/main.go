package main

import (
	"archivus/internal/config"
	"archivus/internal/store"
	"archivus/shell"
)

func main() {
	if err := config.Init(); err != nil {
		panic(err)
	}

	s := store.Store{}
	err := s.Init()
	if err != nil {
		panic(err)
	}

	sh := shell.Shell{Store: &s}
	sh.SetupDrive()
}
