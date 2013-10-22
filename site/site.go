package site

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dchest/fsnotify"

	"github.com/dchest/kkr/assets"
	"github.com/dchest/kkr/filters"
	"github.com/dchest/kkr/layouts"
	"github.com/dchest/kkr/utils"
)

const (
	ConfigFileName = "site.yml"
	AssetsFileName = "assets.yml"

	OutDirName     = "out"
	PostsDirName   = "posts"
	PagesDirName   = "pages"
	LayoutsDirName = "layouts"

	DefaultPermalink = "blog/:year/:month/:day/:name/"

	DefaultPostLayout = "post"
	DefaultPageLayout = "default"
)

type Config struct {
	// Loadable from YAML.
	Name       string                 `yaml:"name"`
	Author     string                 `yaml:"author"`
	Permalink  string                 `yaml:"permalink"`
	URL        string                 `yaml:"url"`
	Filters    map[string]interface{} `yaml:"filters"`
	Properties map[string]interface{} `yaml:"properties"`

	// Generated.
	Date  time.Time
	Posts Posts `yaml:"-"`
	//TODO Categories
}

func readConfig(filename string) (*Config, error) {
	var c Config
	if err := utils.UnmarshallYAMLFile(filename, &c); err != nil {
		return nil, err
	}
	// Set defaults.
	if c.Permalink == "" {
		c.Permalink = DefaultPermalink
	}
	// Some cleanup.
	c.URL = utils.StripEndSlash(c.URL)
	return &c, nil
}

type Site struct {
	BaseDir     string
	Config      *Config
	Assets      *assets.Collection
	Layouts     *layouts.Collection
	PageFilters *filters.Collection

	buildQueue  chan bool
	buildErrors chan error

	watcher             *fsnotify.Watcher
	cleanBeforeBuilding bool
}

func Open(dir string) (s *Site, err error) {
	s = &Site{
		BaseDir:     dir,
		buildQueue:  make(chan bool),
		buildErrors: make(chan error),
	}
	if err := s.LoadConfig(); err != nil {
		return nil, err
	}
	if err := s.LoadPageFilters(); err != nil {
		return nil, err
	}
	if err := s.LoadAssets(); err != nil {
		return nil, err
	}
	// Launch builder goroutine.
	go func() {
		for {
			do := <-s.buildQueue
			if !do {
				return
			}
			s.buildErrors <- s.runBuild()
		}
	}()
	return s, nil
}

func (s *Site) LoadConfig() error {
	conf, err := readConfig(filepath.Join(s.BaseDir, ConfigFileName))
	if err != nil {
		return err
	}
	s.Config = conf
	return nil
}

func (s *Site) LoadAssets() error {
	// Load assets.
	assets, err := assets.Load(AssetsFileName)
	if err != nil {
		return err
	}
	s.Assets = assets
	return nil
}

func (s *Site) LoadPageFilters() error {
	// Load page filters.
	pageFilters := filters.NewCollection()
	for extension, line := range s.Config.Filters {
		if err := pageFilters.AddFromYAML(extension, line); err != nil {
			return err
		}
	}
	s.PageFilters = pageFilters
	return nil
}

// isIgnoredFile returns true if filename should be ignored
// when reading posts and pages (or copying).
func (s *Site) isIgnoredFile(filename string) bool {
	// Files ending with ~ are considered temporary.
	if filename[len(filename)-1] == '~' {
		return true
	}
	// Crap from OS X Finder.
	if filename == ".DS_Store" {
		return true
	}
	return false
}

func (s *Site) LoadPosts() (err error) {
	postsDir := filepath.Join(s.BaseDir, PostsDirName)
	posts := make(Posts, 0)
	err = filepath.Walk(postsDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relname, err := filepath.Rel(postsDir, path)
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		if s.isIgnoredFile(relname) {
			return nil // skip ignored files
		}
		if !isPostFileName(relname) {
			return nil
		}
		p, err := LoadPost(postsDir, relname, s.Config.Permalink)
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
	s.Config.Posts = posts
	return nil
}

func (s *Site) RenderPost(p *Post) error {
	// Render post.
	data, err := s.Layouts.RenderPage(p, DefaultPostLayout)
	if err != nil {
		return err
	}
	log.Printf("B > %s\n", filepath.Join(OutDirName, p.Filename))
	// Apply filter.
	data, err = s.PageFilters.ApplyFilter(filepath.Ext(p.Filename), data)
	if err != nil {
		return err
	}
	// Write to file.
	return utils.WriteStringToFile(filepath.Join(s.BaseDir, OutDirName, p.Filename), data)
}

func (s *Site) RenderPosts() error {
	for _, p := range s.Config.Posts {
		if err := s.RenderPost(p); err != nil {
			return err
		}
	}
	return nil
}

func (s *Site) RenderPage(pagesDir, relname string) error {
	p, err := LoadPage(pagesDir, relname)
	if err != nil {
		if IsNotPage(err) {
			// Not a page, copy file.
			return s.CopyFile(relname)
		}
		return err
	}
	// Render page.
	data, err := s.Layouts.RenderPage(p, DefaultPageLayout)
	if err != nil {
		return err
	}
	log.Printf("P %s → %s\n", relname, filepath.Join(OutDirName, p.Filename))
	// Apply filter.
	data, err = s.PageFilters.ApplyFilter(filepath.Ext(p.Filename), data)
	if err != nil {
		return err
	}
	// Write to file.
	return utils.WriteStringToFile(filepath.Join(s.BaseDir, OutDirName, p.Filename), data)
}

func (s *Site) RenderPages() error {
	inDir := filepath.Join(s.BaseDir, PagesDirName)
	return filepath.Walk(inDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relname, err := filepath.Rel(inDir, path)
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil // TODO(dchest): create directories?
		}
		if s.isIgnoredFile(filepath.Base(relname)) {
			return nil // skip ignored files
		}
		return s.RenderPage(inDir, relname)
	})
}

func (s *Site) CopyFile(filename string) error {
	inDir := filepath.Join(s.BaseDir, PagesDirName)
	outDir := filepath.Join(s.BaseDir, OutDirName)
	if err := os.MkdirAll(filepath.Join(outDir, filepath.Dir(filename)), 0755); err != nil {
		return err
	}
	inFile := filepath.Join(inDir, filename)
	outFile := filepath.Join(outDir, filename)

	// Remove old outfile, ignoring errors.
	os.Remove(outFile)

	// Try making hard link instead of copying.
	if err := os.Link(inFile, outFile); err == nil {
		// Succeeded.
		log.Printf("H %s → %s\n", filename, filepath.Join(OutDirName, filename))
		return nil
	}

	// Failed to create hard link, so try copying content.
	in, err := os.Open(inFile)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	log.Printf("C %s → %s\n", filename, filepath.Join(OutDirName, filename))
	return nil
}

func (s *Site) LoadLayouts() (err error) {
	s.Layouts = layouts.NewCollection(s)
	return s.Layouts.AddDir(filepath.Join(s.BaseDir, LayoutsDirName))
}

func (s *Site) runBuild() (err error) {
	if s.cleanBeforeBuilding {
		s.Clean()
	}
	// Set site build time.
	s.Config.Date = time.Now()
	// Process assets.
	err = s.Assets.Process(filepath.Join(s.BaseDir, OutDirName))
	if err != nil {
		return
	}
	// Load layouts.
	err = s.LoadLayouts()
	if err != nil {
		return
	}
	// Load and render posts.
	err = s.LoadPosts()
	if err != nil {
		return
	}
	err = s.RenderPosts()
	if err != nil {
		return
	}
	// Render pages.
	err = s.RenderPages()
	if err != nil {
		return
	}
	return nil
}

func (s *Site) Build() error {
	t := time.Now()
	defer func() {
		log.Printf("* Build in %s", time.Now().Sub(t))
	}()

	s.buildQueue <- true
	return <-s.buildErrors
}

func (s *Site) Clean() error {
	// Remove output directory.
	log.Printf("* Cleaning.")
	return os.RemoveAll(filepath.Join(s.BaseDir, OutDirName))
}

func (s *Site) LayoutData() interface{} {
	return *s.Config
}

func (s *Site) LayoutFuncs() layouts.FuncMap {
	// TODO cache this map.
	return layouts.FuncMap{
		// `xml` function escapes XML.
		"xml": func(s string) (string, error) {
			var buf bytes.Buffer
			if err := xml.EscapeText(&buf, []byte(s)); err != nil {
				return "", err
			}
			return buf.String(), nil
		},
		// `asset` function returns asset URL by its name.
		"asset": func(name string) (string, error) {
			a := s.Assets.Get(name)
			if a == nil {
				return "", fmt.Errorf("asset %q not found", name)
			}
			return a.Filename, nil
		},
	}
}

func (s *Site) Serve(addr string) error {
	outDir := filepath.Join(s.BaseDir, OutDirName)
	log.Printf("Serving at %s. Press Ctrl+C to quit.\n", addr)
	return http.ListenAndServe(addr, http.FileServer(http.Dir(outDir)))
}

func (s *Site) getWatchedDirs() (dirs []string, err error) {
	// Watch every subdirectory of site except for output directory.
	outDir := filepath.Join(s.BaseDir, OutDirName)
	dirs = make([]string, 0)
	err = filepath.Walk(s.BaseDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			return nil // skip non-directories
		}
		if path == outDir {
			return filepath.SkipDir // skip out directory and its subdirectories
		}
		dirs = append(dirs, path)
		return nil
	})
	return
}

func (s *Site) isWatcherIgnored(name string) bool {
	if filepath.Base(name) == OutDirName {
		return true
	}
	return false
}

func (s *Site) StartWatching() (err error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case ev := <-watcher.Event:
				if s.isWatcherIgnored(ev.Name) {
					break
				}
				log.Println("W event:", ev)
				if err := s.Build(); err != nil {
					log.Printf("! build error: %s", err)
				}
			case err := <-watcher.Error:
				log.Println("! watcher error:", err)
			}
		}
	}()

	watchedDirs, err := s.getWatchedDirs()
	if err != nil {
		return err
	}
	for _, dir := range watchedDirs {
		if err := watcher.Watch(dir); err != nil {
			return err
		}
	}

	s.watcher = watcher

	log.Printf("* Watching for changes.")
	return nil
}

func (s *Site) StopWatching() {
	if s.watcher != nil {
		s.watcher.Close()
		s.watcher = nil
	}
}

func (s *Site) SetCleanBeforeBuilding(clean bool) {
	s.cleanBeforeBuilding = clean
}
