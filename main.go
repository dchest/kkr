package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime/pprof"

	//"github.com/dchest/fsnotify"

	"github.com/dchest/kkr/site"
)

var currentSite *site.Site

func serve(dir string) {
	log.Printf("Serving at %s. Press Ctrl+C to quit.\n", *fHttp)
	log.Fatal(http.ListenAndServe(*fHttp, http.FileServer(http.Dir(dir))))
}

/*
func getWatchedDirs(basedir string) (dirs []string, err error) {
	// Watch every subdirectory of site except for _out dir.
	outdir := filepath.Join(basedir, outDirName)
	dirs = make([]string, 0)
	err = filepath.Walk(basedir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			return nil // skip non-directories
		}
		if path == outdir {
			return filepath.SkipDir // skip out directory and its subdirectories
		}
		dirs = append(dirs, path)
		return nil
	})
	return
}

func isWatcherIgnored(name string) bool {
	if filepath.Base(name) == hashCacheFileName {
		return true
	}
	if filepath.Base(name) == outDirName {
		return true
	}
	return false
}

func startWatcher(basedir string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	go func(basedir string) {
		for {
			select {
			case ev := <-watcher.Event:
				if isWatcherIgnored(ev.Name) {
					break
				}
				log.Println("W event:", ev)
				build(basedir)
			case err := <-watcher.Error:
				log.Println("! Watcher error:", err)
			}
		}
	}(basedir)

	watchedDirs, err := getWatchedDirs(basedir)
	if err != nil {
		return nil, err
	}

	for _, dir := range watchedDirs {
		err = watcher.Watch(dir)
		if err != nil {
			return nil, err
		}
	}

	log.Printf("* Watching for changes.")
	return watcher, nil
}
*/

var (
	fHttp       = flag.String("http", "localhost:8080", "address and port to use for serving")
	fWatch      = flag.Bool("watch", false, "watch for changes")
	fNoFilters  = flag.Bool("nofilters", false, "disable filters")
	fNoRemove   = flag.Bool("noremove", false, "don't delete output directory before building")
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

	/*
	var watcher *fsnotify.Watcher
	if *fWatch {
		watcher, err = startWatcher(wd)
		if err != nil {
			log.Fatalf("! Cannot start watcher: %s", err)
		}
	}
	*/

	switch command {
	case "build":
		err = currentSite.Build()
		if err != nil {
			log.Printf("! build error: %s", err)
		}
		//if watcher != nil {
		//	select {}
		//}
	case "serve":
		err = currentSite.Build()
		if err != nil {
			log.Printf("! build error: %s", err)
		}
		serve(dir + "/_out") //XXX
	case "clean":
		err = currentSite.Clean()
		if err != nil {
			log.Printf("! clean error: %s", err)
		}
	default:
		log.Printf("! unknown command %s", command)
		flag.Usage()
	}
	/*
	if watcher != nil {
		watcher.Close()
	}
	*/
}
