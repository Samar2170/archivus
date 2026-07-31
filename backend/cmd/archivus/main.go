package main

import (
	"archivus/internal/config"
	"archivus/internal/services/auth"
	"archivus/internal/services/storagemanager"
	"archivus/internal/store"
	"archivus/server"
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

	err = parser.Parse(os.Args)
	if err != nil {
		print(parser.Usage(err))
		return
	}
	fmt.Printf("Running in %s mode\n", *serverMode)
	if err := config.Init(*serverMode); err != nil {
		panic(err)
	}
	fmt.Printf("Config initialized\n")
	fmt.Println(config.Config)

	s, err := store.GetStore(config.ProjectBaseDir)
	if err != nil {
		panic(err)
	}
	dm, err := storagemanager.FromConfig(s)
	if err != nil {
		panic(err)
	}
	as := auth.AuthService{
		Store:              s,
		StorageManager:     dm,
		DefaultWriteAccess: config.Config.DefaultWriteAccess,
		SecretKey:          config.Config.SecretKey,
	}

	switch {
	case serverCmd.Happened():
		server := server.GetServer(&as)
		if err := server.ListenAndServe(); err != nil {
			panic(err)
		}

	default:
		print(parser.Usage(nil))
	}

}
