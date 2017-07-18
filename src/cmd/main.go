package main

import (
	"api"
	"flag"
	"fmt"
	"log"
	"migrate"
	"net/http"
	"os"
)

var (
	listenAddress   = flag.String("listen-address", "127.0.0.1:8080", "bind address for http-api")
	connectionInfo  = flag.String("connection-string", "postgresql://127.0.0.1/storage?user=storage&password=storage&sslmode=disable", "postgres connection info")
	workDir         = flag.String("workdir", "/tmp", "path to disk temporary directory")
	runMigrateOnly  = flag.Bool("run-migrate", false, "run migrate scripts and exit")
	showVersionOnly = flag.Bool("v", false, "prints current version")
	BuildVersion    = `UNKNOWN`
)

func main() {

	if !flag.Parsed() {
		flag.Parse()
	}

	if *showVersionOnly {
		fmt.Printf("%s\n", BuildVersion)
		os.Exit(1)
	}

	if *runMigrateOnly {
		log.Printf("[INFO] migrate starting\n")
		if err := migrate.Run(*connectionInfo); err != nil {
			log.Printf("[ERROR] run migrate: %s\n", err.Error())
			os.Exit(7)
		} else {
			log.Printf("[INFO] migrate completed successfully\n")
			os.Exit(0)
		}
	}

	st, err := api.New(*workDir, *connectionInfo)
	if err != nil {
		log.Printf("[FATAL] create api: %s\n", err.Error())
		os.Exit(2)
	}

	finish := make(chan error)
	go func() {
		log.Printf("[INFO] starting api at %s\n", *listenAddress)
		finish <- http.ListenAndServe(*listenAddress, st)
	}()

	finishErr := <-finish
	if finishErr != nil {
		log.Printf("[ERROR] listen server: %s\n", finishErr.Error())
		os.Exit(3)
	}

	log.Printf("[INFO] shutdown\n")
	os.Exit(0)
}
