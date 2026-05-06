package main

import (
	"archivus/internal/store"
	"archivus/shell"
)

func main() {
	s := store.Store{}
	err := s.Init()
	if err != nil {
		panic(err)
	}

	sh := shell.Shell{Store: &s}
	sh.SetupDrive()
}
