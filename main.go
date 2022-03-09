// Copyright 2013 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/pprof"

	"github.com/dchest/kkr/site"
	"github.com/dchest/kkr/utils"
)

var currentSite *site.Site

var (
	fHttp  = flag.String("http", "localhost:8080", "address and port to use for serving")
	fWatch = flag.Bool("watch", false, "watch for changes")
	//fNoFilters  = flag.Bool("nofilters", false, "disable filters")
	fNoClean    = flag.Bool("noclean", false, "don't delete output directory before building")
	fCPUProfile = flag.String("cpuprofile", "", "(debug) write CPU profile to file")
	fNoCache    = flag.Bool("nocache", false, "disables caching when watching")
	fBrowser    = flag.Bool("browser", false, "open local site in browser after starting the web server")
)

var Usage = func() {
	fmt.Printf(`usage: kkr command [options]

Commands:
  build  - build website
  serve  - start a web server
  dev    - same as "serve -watch -browser"
  clean  - clean caches and remove output directory

Options:
`)
	flag.PrintDefaults()
}

func main() {
	log.SetFlags(0)
	flag.Usage = Usage

	var command string

	if len(os.Args) < 2 {
		flag.Usage()
		return
	}
	command = os.Args[1]
	os.Args = os.Args[1:]

	watch := *fWatch || command == "dev"

	flag.Parse()

	if *fCPUProfile != "" {
		f, err := os.Create(*fCPUProfile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("! os.Getwd(): %s", err)
	}
	currentSite, err = site.Open(dir)
	if err != nil {
		log.Fatalf("! Cannot open site: %s", err)
	}
	if watch {
		if !*fNoCache {
			site.EnableCache(true)
			// XXX Layouts cache is disabled until I write
			// new code that works when parent layout changes.
			//layouts.EnableCache(true)
		}
		if err := currentSite.StartWatching(); err != nil {
			log.Fatalf("! Cannot start watcher: %s", err)
		}
	}
	currentSite.SetCleanBeforeBuilding(!*fNoClean)

	switch command {
	case "build":
		err = currentSite.Build()
		if err != nil {
			log.Printf("! build error: %s", err)
		}
		if watch {
			log.Printf("Watching for changes. Press Ctrl+C to quit.")
			select {}
		}
	case "serve", "dev":
		if command == "dev" {
			currentSite.SetDevMode(true)
		}
		serverDone := make(chan bool)
		go func() {
			err := currentSite.Serve(*fHttp)
			if err != nil {
				log.Fatalf("! serving error: %s", err)
			}
			serverDone <- true
		}()
		err = currentSite.Build()
		if err != nil {
			log.Fatalf("! build error: %s", err)
		}
		if *fBrowser || command == "dev" {
			if err := utils.OpenURL("http://" + *fHttp); err != nil {
				log.Printf("! cannot open browser: %s", err)
			}
		}
		<-serverDone
	case "clean":
		err = currentSite.Clean()
		if err != nil {
			log.Printf("! clean error: %s", err)
		}
	default:
		log.Printf("! unknown command %s", command)
		flag.Usage()
	}
	if watch {
		currentSite.StopWatching()
	}
}
