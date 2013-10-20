package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"

	"github.com/dchest/fsnotify"
	"github.com/dchest/goyaml"

	"github.com/dchest/kkr/assets"
	"github.com/dchest/kkr/filters"
	"github.com/dchest/kkr/hashcache"
	"github.com/dchest/kkr/layout"
)

const (
	layoutsDirName    = "_layouts"
	pagesDirName      = "_pages"
	postsDirName      = "_posts"
	outDirName        = "_out"
	assetsDirName     = "_assets" // this is only used in watcher
	siteFileName      = "_config.yml"
	assetsFileName    = "_assets.yml"
	hashCacheFileName = ".kkr-hashcache"

	defaultPermalink = "blog/:year/:month/:day/:name/"

	defaultPageLayout = "default"
	defaultPostLayout = "post"
)

var site map[string]interface{}
var hcache *hashcache.Cache

var postExtensions = []string{".html", ".htm", ".md", ".markdown"}

func isPostFileName(filename string) bool {
	ext := filepath.Ext(filename)
	for _, v := range postExtensions {
		if v == ext {
			return true
		}
	}
	return false
}

// isIgnoredFile returns true if filename should be ignored
// when reading posts and pages.
func isIgnoredFile(filename string) bool {
	if filename[len(filename)-1] == '~' {
		return true
	}
	return false
}

func loadLayouts(basedir string) error {
	return layout.AddDir(filepath.Join(basedir, layoutsDirName))
}

func loadSiteConfig(basedir string) error {
	// Fill in default values.
	//TODO move more defaults from contants to here.
	site = make(map[string]interface{})
	site["permalink"] = defaultPermalink
	site["date"] = time.Now()

	// Read config file.
	b, err := ioutil.ReadFile(filepath.Join(basedir, siteFileName))
	if err != nil {
		return err
	}
	if err := goyaml.Unmarshal(b, &site); err != nil {
		return err
	}

	// Some cleanup.
	if url, ok := site["url"]; ok {
		site["url"] = cleanSiteURL(url.(string))
	}

	// Register filters.
	fv, ok := site["filters"]
	if ok {
		switch fs := fv.(type) {
		case map[interface{}]interface{}:
			for k, v := range fs {
				ext := k.(string)
				switch name := v.(type) {
				case string:
					if err := filters.RegisterExt(ext, name, nil); err != nil {
						return err
					}
				case []interface{}:
					args := make([]string, len(name))
					for i, a := range name {
						args[i] = a.(string)
					}
					if err := filters.RegisterExt(ext, args[0], args[1:]); err != nil {
						return err
					}
				default:
					return fmt.Errorf("unknown filter format type")
				}
			}
		default:
			return fmt.Errorf("'filters' config is not a map")
		}
	}
	return nil
}

func copyFile(basedir string, filename string) error {
	indir := filepath.Join(basedir, pagesDirName)
	outdir := filepath.Join(basedir, outDirName)
	if err := os.MkdirAll(filepath.Join(outdir, filepath.Dir(filename)), 0755); err != nil {
		return err
	}
	infile := filepath.Join(indir, filename)
	outfile := filepath.Join(outdir, filename)

	// Remove old outfile, ignoring errors.
	os.Remove(outfile)

	// Try making hard link instead of copying.
	if err := os.Link(infile, outfile); err == nil {
		// Succeeded.
		log.Printf("H %s → %s\n", filename, filepath.Join(outDirName, filename))
		return nil
	}

	// Failed to create hard link, so try copying content.
	in, err := os.Open(filepath.Join(indir, filename))
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(filepath.Join(outdir, filename))
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	log.Printf("C %s → %s\n", filename, filepath.Join(outDirName, filename))
	return nil
}

func renderPages(basedir string) error {
	indir := filepath.Join(basedir, pagesDirName)
	outdir := filepath.Join(basedir, outDirName)
	return filepath.Walk(indir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relname, err := filepath.Rel(indir, path)
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil // TODO(dchest): create directories?
		}
		if isIgnoredFile(relname) {
			return nil // skip ignored files
		}
		p, err := LoadPage(indir, relname)
		if err != nil && IsNotPage(err) {
			// Not a page, copy file.
			return copyFile(basedir, relname)
		}
		if err != nil {
			return err
		}
		// Render templated page.
		log.Printf("P %s → %s\n", relname, filepath.Join(outDirName, p.Filename))
		l, err := layout.New("", defaultPageLayout, p.Meta, p.Content)
		if err != nil {
			return err
		}
		rendered, err := l.Render(site, p.Meta, "")
		if err != nil {
			return err
		}
		// Filters.
		filtered, filterName, err := filters.FilterTextByExt(filepath.Ext(p.Filename), rendered)
		if err != nil {
			return err
		}
		if filterName != "" {
			log.Printf("  | filter: %s\n", filterName)
		}
		//if hcache.Seen(filepath.Join(outDirName, p.Filename), filtered) {
		//	log.Println("  | unchanged")
		//	return nil
		//}
		outpath := filepath.Join(outdir, p.Filename)
		if err := os.MkdirAll(filepath.Dir(outpath), 0755); err != nil {
			return err
		}
		if err := ioutil.WriteFile(outpath, []byte(filtered), 0644); err != nil {
			return err
		}
		return nil
	})
}

func loadPosts(basedir string) error {
	indir := filepath.Join(basedir, postsDirName)
	posts := make(Posts, 0)
	err := filepath.Walk(indir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relname, err := filepath.Rel(indir, path)
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil // TODO(dchest): create directories?
		}
		if isIgnoredFile(relname) {
			return nil // skip ignored files
		}
		if !isPostFileName(relname) {
			return nil
		}
		p, err := LoadPost(indir, relname, site["permalink"].(string))
		if err != nil {
			return err
		}
		posts = append(posts, p)
		log.Printf("B < %s\n", relname)
		return nil
	})
	if err != nil {
		return err
	}
	posts.Sort()
	site["Posts"] = posts
	return nil
}

func renderPosts(basedir string) error {
	outdir := filepath.Join(basedir, outDirName)
	posts, ok := site["Posts"]
	if !ok || posts == nil {
		return nil
	}
	for _, p := range posts.(Posts) {
		// Render post.
		log.Printf("B > %s\n", filepath.Join(outDirName, p.Filename))
		l, err := layout.New("", defaultPostLayout, p.Meta, p.Content)
		if err != nil {
			return err
		}
		rendered, err := l.Render(site, p.Meta, "")
		if err != nil {
			return err
		}
		// Filters.
		filtered, filterName, err := filters.FilterTextByExt(filepath.Ext(p.Filename), rendered)
		if err != nil {
			return err
		}
		if filterName != "" {
			log.Printf("  | filter: %s\n", filterName)
		}
		//if hcache.Seen(filepath.Join(outDirName, p.Filename), filtered) {
		//	log.Println("  | unchanged")
		//	return nil
		//}
		if err := os.MkdirAll(filepath.Join(outdir, filepath.Dir(p.Filename)), 0755); err != nil {
			return err
		}
		if err := ioutil.WriteFile(filepath.Join(outdir, p.Filename), []byte(filtered), 0644); err != nil {
			return err
		}
	}
	return nil
}

func isDirExist(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
}

func clean(wd string) {
	os.Remove(filepath.Join(wd, hashCacheFileName))
	if !*fNoRemove {
		if err := os.RemoveAll(filepath.Join(wd, outDirName)); err != nil {
			log.Printf("! Error removing: %s. Continuing...", err)
		}
	}
}

func build(wd string) {
	startTime := time.Now()

	log.Println("* Building:")

	// Load hashcache.
	//var err error
	//hcache, err = hashcache.Open(hashCacheFileName)
	//if err != nil {
	//	log.Fatalf("! Cannot load or create hashcache, please delete %q", hashCacheFileName)
	//}
	// Clean cache if _out dir doesn't exist.
	if !isDirExist(filepath.Join(wd, outDirName)) {
		log.Printf("* Cleaned hashcache.")
		hcache.Clean()
	} else {
		// Remove _out.
		if !*fNoRemove {
			log.Printf("* Removing %q\n", outDirName)
			if err := os.RemoveAll(filepath.Join(wd, outDirName)); err != nil {
				log.Printf("! Error removing: %s. Continuing...", err)
			}
		}
	}

	// Load and process assets.
	if err := assets.LoadAssets(filepath.Join(wd, assetsFileName)); err != nil {
		log.Fatalf("! Cannot load assets: %s", err)
	}
	if err := assets.ProcessAssets(filepath.Join(wd, outDirName)); err != nil {
		log.Fatalf("! Error processing assets: %s", err)
	}

	// Load layouts.
	if err := loadLayouts(wd); err != nil {
		log.Fatalf("! Cannot load layouts: %s", err)
	}

	// Load and render posts.
	if isDirExist(filepath.Join(wd, postsDirName)) {
		if err := loadPosts(wd); err != nil {
			log.Fatalf("! Cannot load posts: %s", err)
		}
		if err := renderPosts(wd); err != nil {
			log.Fatalf("! Cannot render post: %s", err)
		}
	} else {
		log.Println("- No posts to render.")
	}

	// Render pages.
	if isDirExist(filepath.Join(wd, pagesDirName)) {
		if err := renderPages(wd); err != nil {
			log.Fatalf("! Cannot render page: %s", err)
		}
	} else {
		log.Println("- No pages to render.")
	}

	// Save hashcache.
	//if err := hcache.Save(); err != nil {
	//	log.Fatalf("! Cannot save hashcache")
	//}

	log.Printf("* Done in %s\n", time.Now().Sub(startTime))
}

func serve(wd string) {
	outdir := filepath.Join(wd, outDirName)
	log.Printf("Serving at %s. Press Ctrl+C to quit.\n", *fHttp)
	log.Fatal(http.ListenAndServe(*fHttp, http.FileServer(http.Dir(outdir))))
}

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

var (
	fHttp       = flag.String("http", "localhost:8080", "address and port to use for serving")
	fWatch      = flag.Bool("watch", false, "watch for changes")
	fNoFilters  = flag.Bool("nofilters", false, "disable filters")
	fNoRemove   = flag.Bool("noremove", false, "don't delete " + outDirName + " before building")
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

	filters.SetEnabled(!*fNoFilters)

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("! os.Getwd(): %s", err)
	}
	if err := loadSiteConfig(wd); err != nil {
		log.Fatalf("! Cannot load site config: %s", err)
	}

	var watcher *fsnotify.Watcher
	if *fWatch {
		watcher, err = startWatcher(wd)
		if err != nil {
			log.Fatalf("! Cannot start watcher: %s", err)
		}
	}

	switch command {
	case "build":
		build(wd)
		if watcher != nil {
			select {}
		}
	case "serve":
		build(wd)
		serve(wd)
	case "clean":
		clean(wd)
	default:
		log.Printf("! unknown command %s", command)
		flag.Usage()
	}
	if watcher != nil {
		watcher.Close()
	}
}
