package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"storage"
)

var (
	listenAddress   = flag.String("listen-address", "127.0.0.1:8080", "bind address for http-api")
	connectionInfo  = flag.String("connection-string", "postgresql://127.0.0.1/storage?user=storage&password=storage&sslmode=disable", "postgres connection info")
	workDir         = flag.String("workdir", "/tmp", "path to disk temporary directory")
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

	st, err := storage.New(*workDir, *connectionInfo)
	if err != nil {
		log.Printf("[FATAL] open storage: %s\n", err.Error())
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
