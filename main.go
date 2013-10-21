package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/pprof"

	"github.com/dchest/kkr/site"
)

var currentSite *site.Site

var (
	fHttp       = flag.String("http", "localhost:8080", "address and port to use for serving")
	fWatch      = flag.Bool("watch", false, "watch for changes")
	//fNoFilters  = flag.Bool("nofilters", false, "disable filters")
	//fNoRemove   = flag.Bool("noremove", false, "don't delete output directory before building")
	fCPUProfile = flag.String("cpuprofile", "", "(debug) write CPU profile to file")
)

var Usage = func() {
	fmt.Printf(`usage: kkr command [options]

Commands:
  build  - build website
  serve  - start a web server
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
	if *fWatch {
		if err := currentSite.StartWatching(); err != nil {
			log.Fatalf("! Cannot start watcher: %s", err)
		}
	}

	switch command {
	case "build":
		err = currentSite.Build()
		if err != nil {
			log.Printf("! build error: %s", err)
		}
		if *fWatch {
			log.Printf("Watching for changes. Press Ctrl+C to quit.")
			select {}
		}
	case "serve":
		err = currentSite.Build()
		if err != nil {
			log.Printf("! build error: %s", err)
		}
		err = currentSite.Serve(*fHttp)
		if err != nil {
			log.Fatalf("! serving error: %s", err)
		}
	case "clean":
		err = currentSite.Clean()
		if err != nil {
			log.Printf("! clean error: %s", err)
		}
	default:
		log.Printf("! unknown command %s", command)
		flag.Usage()
	}
	if *fWatch {
		currentSite.StopWatching()
	}
}
