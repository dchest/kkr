// Copyright 2013 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package site implements everything related to building a site.
package site

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"unicode/utf8"

	"github.com/dchest/static-search/indexer"

	"github.com/dchest/kkr/assets"
	"github.com/dchest/kkr/filters"
	"github.com/dchest/kkr/fspoll"
	"github.com/dchest/kkr/layouts"
	"github.com/dchest/kkr/markup"
	"github.com/dchest/kkr/utils"
)

const (
	ConfigFileName = "site.yml"
	AssetsFileName = "assets.yml"

	AssetsDirName   = "assets" // just a convention, currently used for watching only
	IncludesDirName = "includes"
	LayoutsDirName  = "layouts"
	PagesDirName    = "pages"
	PostsDirName    = "posts"
	OutDirName      = "out"

	DefaultPermalink = "blog/:year/:month/:day/:name/"

	DefaultPostLayout = "post"
	DefaultPageLayout = "default"
)

var (
	HTMLExtensions     = []string{".html", ".htm"}
	MarkdownExtensions = []string{".markdown", ".md"}
	PostExtensions     = []string{".html", ".htm", ".markdown", ".md"}
)

type Config struct {
	// Loadable from YAML.
	Name        string                 `yaml:"name"`
	Author      string                 `yaml:"author"`
	Permalink   string                 `yaml:"permalink"`
	URL         string                 `yaml:"url"`
	Filters     map[string]interface{} `yaml:"filters"`
	Properties  map[string]interface{} `yaml:"properties"`
	SearchIndex string                 `yaml:"search_index"`
	Markup      *markup.Options        `yaml:"markup"`

	// Generated.
	Date    time.Time
	Posts   Posts            `yaml:"-"`
	Tags    map[string]Posts `yaml:"-"`
	TagList []string         `yaml:"-"`
}

func (c Config) PostsByTag(tagName string) Posts {
	return c.Tags[tagName]
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
	Includes    map[string]string

	buildQueue  chan bool
	buildErrors chan error

	watcher             *fspoll.Watcher
	cleanBeforeBuilding bool
}

func Open(dir string) (s *Site, err error) {
	s = &Site{
		BaseDir:     dir,
		buildQueue:  make(chan bool),
		buildErrors: make(chan error),
	}
	// Try loading config.
	if err := s.LoadConfig(); err != nil {
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
	log.Printf("* Loading assets.")
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

func (s *Site) LoadLayouts() (err error) {
	log.Printf("* Loading layouts.")
	s.Layouts = layouts.NewCollection(s)
	return s.Layouts.AddDir(filepath.Join(s.BaseDir, LayoutsDirName))
}

func (s *Site) LoadIncludes() (err error) {
	log.Printf("* Loading includes.")
	s.Includes = make(map[string]string)
	includesDir := filepath.Join(s.BaseDir, IncludesDirName)
	err = filepath.Walk(includesDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relname, err := filepath.Rel(includesDir, path)
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		log.Printf("I %s", relname)
		b, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		s.Includes[relname] = string(b)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return err
	}
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
	log.Printf("* Loading posts.")
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
		if !utils.HasFileExt(relname, PostExtensions) {
			return nil
		}
		log.Printf("B < %s\n", relname)
		p, err := LoadPost(postsDir, relname, s.Config.Permalink)
		if err != nil {
			return err
		}
		posts = append(posts, p)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	// Sort and add to config.
	posts.Sort()
	s.Config.Posts = posts
	// Distribute by tags.
	tags := make(map[string]Posts)
	for _, p := range posts {
		for _, tagName := range p.Tags {
			tags[tagName] = append(tags[tagName], p)
		}
	}
	tagList := make([]string, 0, len(tags))
	for tagName := range tags {
		tagList = append(tagList, tagName)
	}
	s.Config.TagList = tagList
	s.Config.Tags = tags
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
	log.Printf("* Rendering posts.")
	for _, p := range s.Config.Posts {
		if err := s.RenderPost(p); err != nil {
			return err
		}
	}
	return nil
}

func (s *Site) RenderPage(pagesDir, relname string) error {
	log.Printf("P < %s\n", relname)
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
	log.Printf("P > %s\n", filepath.Join(OutDirName, p.Filename))
	// Apply filter.
	data, err = s.PageFilters.ApplyFilter(filepath.Ext(p.Filename), data)
	if err != nil {
		return err
	}
	// Write to file.
	return utils.WriteStringToFile(filepath.Join(s.BaseDir, OutDirName, p.Filename), data)
}

func (s *Site) RenderPages() error {
	log.Printf("* Rendering pages")
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
		log.Printf("H > %s\n", filepath.Join(OutDirName, filename))
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
	log.Printf("C > %s\n", filepath.Join(OutDirName, filename))
	return nil
}

func (s *Site) runBuild() (err error) {
	if s.cleanBeforeBuilding {
		err = s.Clean()
		if err != nil {
			return
		}
	}
	// Reload config.
	if err := s.LoadConfig(); err != nil {
		return err
	}
	// Set site build time.
	s.Config.Date = time.Now()
	// Set markup options
	markup.SetOptions(s.Config.Markup)
	// Load page filters.
	if err := s.LoadPageFilters(); err != nil {
		return err
	}
	// Load assets.
	if err := s.LoadAssets(); err != nil {
		return err
	}
	// Load includes.
	if err := s.LoadIncludes(); err != nil {
		return err
	}
	// Process assets.
	log.Printf("* Processing assets.")
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

func (s *Site) Build() (err error) {
	t := time.Now()

	s.buildQueue <- true
	err = <-s.buildErrors
	if err != nil {
		return err
	}
	if s.Config.SearchIndex != "" {
		s.generateSearchIndex()
	}
	log.Printf("* Built in %s", time.Now().Sub(t))
	return nil
}

func (s *Site) generateSearchIndex() error {
	log.Printf("* Indexing")
	dir := filepath.Clean(filepath.Join(s.BaseDir, OutDirName))
	index := indexer.New()
	n := 0
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !utils.HasFileExt(path, HTMLExtensions) {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		url := utils.CleanPermalink(filepath.ToSlash(path[len(dir):]))
		err = index.AddHTML(url, f)
		f.Close()
		if err != nil {
			return err
		}
		n++
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	f, err := os.Create(filepath.Join(s.BaseDir, OutDirName, s.Config.SearchIndex))
	if err != nil {
		return err
	}
	defer f.Close()
	if n == 0 {
		log.Println("* No documents indexed.")
		return nil
	}
	if _, err := fmt.Fprintf(f, "var searchIndex = "); err != nil {
		return err
	}
	err = index.WriteJSON(f)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("* Indexed %d documents.", n)
	return nil
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
		"xml": func(in string) (string, error) {
			var buf bytes.Buffer
			if err := xml.EscapeText(&buf, []byte(in)); err != nil {
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
			return a.Result, nil
		},
		// `include` function returns text from include file.
		"include": func(name string) (string, error) {
			out, ok := s.Includes[name]
			if !ok {
				return "", fmt.Errorf("include %q not found", name)
			}
			return out, nil
		},
		// `abspaths` adds site URL to relative paths of src and href attributes.
		"abspaths": func(in string) (string, error) {
			return utils.AbsPaths(s.Config.URL, in), nil
		},
		// `truncate` function truncates text to the specified number of bytes.
		// Appends "..." if the string was truncated.
		"truncate": func(n int, s string) (string, error) {
			byteCount := 0
			runeCount := 0
			for byteCount < len(s) {
				_, size := utf8.DecodeRuneInString(s[byteCount:])
				byteCount += size
				runeCount++
				if runeCount >= n {
					return s[:byteCount] + "...", nil
				}
			}
			return s, nil
		},
		// `striptags` removes HTML tags from the given string.
		"striptags": func(s string) (string, error) {
			return utils.StripHTMLTags(s), nil
		},
	}
}

func (s *Site) Serve(addr string) error {
	outDir := filepath.Join(s.BaseDir, OutDirName)
	log.Printf("Serving at %s. Press Ctrl+C to quit.\n", addr)
	return http.ListenAndServe(addr, http.FileServer(http.Dir(outDir)))
}

func (s *Site) StartWatching() (err error) {
	// Watch every subdirectory of site except for output directory and .git.
	excludeGlobs := []string{filepath.Join(s.BaseDir, OutDirName), filepath.Join(s.BaseDir, ".git")}
	watcher, err := fspoll.Watch(s.BaseDir, excludeGlobs, 0)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-watcher.Change:
				log.Println("W detected change")
				if err := s.Build(); err != nil {
					log.Printf("! build error: %s", err)
				}
			case err := <-watcher.Error:
				log.Println("! watcher error:", err)
			}
		}
	}()
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
