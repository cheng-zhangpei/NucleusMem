package main

import (
	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/runtime/memspace"
	"github.com/pingcap-incubator/tinykv/log"
	"os"
)

func main() {
	// todo change the read method of the config file
	// Print working directory
	if wd, err := os.Getwd(); err != nil {
		log.Infof("Failed to get working directory: %v\n", err)
	} else {
		log.Infof("Current working directory: %s\n", wd)
	}
	filepath := "./pkg/configs/file/memspace_1001.yaml"
	config, err := configs.LoadMemSpaceConfigFromYAML(filepath)
	if err != nil {
		panic(err)
	}
	mp, err := memspace.NewMemSpace(config)
	if err != nil {
		panic(err)
	}
	httpServer := memspace.NewMemSpaceHTTPServer(mp)
	err = httpServer.Start()
	if err != nil {
		panic(err)
	}
}
