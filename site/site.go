// Copyright 2013 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package site implements everything related to building a site.
package site

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dchest/kkr/csp"
	"github.com/dchest/kkr/filewriter"
	"github.com/dchest/kkr/search"
	"github.com/dchest/kkr/search/indexer"
	"github.com/dchest/kkr/sitemap"

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
	CSPFileName    = "csp.yml"

	AssetsDirName   = "assets" // just a convention, currently used for watching only
	IncludesDirName = "includes"
	LayoutsDirName  = "layouts"
	PagesDirName    = "pages"
	PostsDirName    = "posts"
	OutDirName      = "out"

	DefaultPermalink = "blog/:year/:month/:day/:name/"

	DefaultPostLayout     = "post"
	DefaultPageLayout     = "default"
	DefaultTagIndexLayout = "tag"
)

var (
	HTMLExtensions     = []string{".html", ".htm"}
	MarkdownExtensions = []string{".markdown", ".md"}
	PostExtensions     = []string{".html", ".htm", ".markdown", ".md"}
)

type SearchConfig struct {
	Index   string   `yaml:"index"`
	Exclude []string `yaml:"exclude"`
}

type TagIndexConfig struct {
	Permalink string `yaml:"permalink"`
	Layout    string `yaml:"layout"`
}

type StaticConfig struct {
	Path   string `yaml:"path"`
	URL    string `yaml:"url"`
	DevURL string `yaml:"dev_url"`
	Assets bool   `yaml:"assets"`
}

type Config struct {
	// Loadable from YAML.
	Name       string                     `yaml:"name"`
	Author     string                     `yaml:"author"`
	Permalink  string                     `yaml:"permalink"`
	URL        string                     `yaml:"url"`
	Static     *StaticConfig              `yaml:"static"`
	Filters    map[string]interface{}     `yaml:"filters"`
	Properties map[string]interface{}     `yaml:"properties"`
	Search     *SearchConfig              `yaml:"search"`
	Markup     *markup.Options            `yaml:"markup"`
	Compress   *filewriter.CompressConfig `yaml:"compress"`
	TagIndex   *TagIndexConfig            `yaml:"tagindex"`
	Sitemap    string                     `yaml:"sitemap"`

	// Generated.
	Date    time.Time
	Posts   Posts            `yaml:"-"`
	Tags    map[string]Posts `yaml:"-"`
	TagList []string         `yaml:"-"`
}

func (c Config) PostsByTag(tag string) Posts {
	return c.Tags[tag]
}

func (c Config) TagURL(tag string) (string, error) {
	if c.TagIndex == nil {
		return "", errors.New("No tagindex in site.yml")
	}
	out := strings.Replace(c.TagIndex.Permalink, ":tag", tag, -1)
	out = strings.Replace(out, ":lctag", strings.ToLower(tag), -1)
	return out, nil
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
	if c.Markup == nil {
		c.Markup = &markup.Options{} // default options
	}
	// Some cleanup.
	c.URL = utils.StripEndSlash(c.URL)
	// Precalculate compressors.
	return &c, nil
}

type Site struct {
	BaseDir     string
	Config      *Config
	Assets      *assets.Collection
	Layouts     *layouts.Collection
	PageFilters *filters.Collection
	CSP         csp.Directives
	Includes    map[string]string

	buildQueue  chan bool
	buildErrors chan error

	watcher             *fspoll.Watcher
	cleanBeforeBuilding bool
	fileWriter          *filewriter.FileWriter
	devMode             bool
	layoutFuncs         layouts.FuncMap
	sitemap             *sitemap.Sitemap
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

func (s *Site) SetDevMode(dev bool) {
	s.devMode = dev
	if !dev {
		s.Config.Compress = nil
		s.fileWriter, _ = filewriter.New(nil)
	}
}

func (s *Site) LoadConfig() error {
	conf, err := readConfig(filepath.Join(s.BaseDir, ConfigFileName))
	if err != nil {
		return err
	}
	compress := conf.Compress
	if s.devMode {
		compress = nil
	}
	s.fileWriter, err = filewriter.New(compress)
	if err != nil {
		return err
	}
	s.Config = conf
	if conf.Sitemap != "" {
		s.sitemap = sitemap.New()
	}
	if s.devMode {
		// In dev mode, override static url with dev_url if it exists.
		if s.Config.Static != nil && s.Config.Static.DevURL != "" {
			s.Config.Static.URL = s.Config.Static.DevURL
		}
	}
	return nil
}

func (s *Site) LoadAssets() error {
	log.Printf("* Loading assets.")
	// Load assets.
	assets, err := assets.Load(AssetsFileName)
	if err != nil {
		return err
	}
	if s.Config.Search != nil && s.Config.Search.Index != "" {
		assets.SetStringAsset("search-script", search.GetSearchScript(s.Config.Search.Index))
	}
	s.Assets = assets
	return nil
}

func (s *Site) LoadCSP() error {
	log.Printf("* Loading CSP.")
	csp, err := csp.Load(CSPFileName)
	if err != nil {
		return err
	}
	s.CSP = csp
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
		for _, tag := range p.Tags {
			// If we have a lowercased tag, but don't have
			// the original-cased tag, normalize it to lowercase;
			// do the same with title-cased tag.
			lowerTag := strings.ToLower(tag)
			titleTag := strings.Title(tag) // deprecated, but we don't care about punctuation
			if _, hasTag := tags[tag]; !hasTag {
				if _, hasLower := tags[lowerTag]; hasLower {
					tag = lowerTag
				} else {
					if _, hasTitle := tags[titleTag]; hasTitle {
						tag = titleTag
					}
				}
			}
			tags[tag] = append(tags[tag], p)
		}
	}
	tagList := make([]string, 0, len(tags))
	for tagName := range tags {
		tagList = append(tagList, tagName)
	}
	sort.Strings(tagList)
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
	b, err := s.PageFilters.ApplyFilter(filepath.Ext(p.Filename), []byte(data))
	if err != nil {
		return err
	}
	if s.sitemap != nil {
		// Add to sitemap.
		if p.InSitemap() {
			if err := s.sitemap.Add(p.SitemapEntry()); err != nil {
				return err
			}
		}
	}
	// Write to file.
	return s.fileWriter.WriteFile(filepath.Join(s.BaseDir, OutDirName, p.Filename), b)
}

func (s *Site) RenderPosts() error {
	log.Printf("* Rendering posts.")
	pool := utils.NewPool()
	for _, v := range s.Config.Posts {
		post := v
		if !pool.Add(func() error { return s.RenderPost(post) }) {
			break
		}
	}
	return pool.Wait()
}

func (s *Site) RenderTagsIndex() error {
	log.Printf("* Rendering tags index")
	pool := utils.NewPool()
	for _, v := range s.Config.TagList {
		tag := v
		if !pool.Add(func() error { return s.RenderTag(tag) }) {
			break
		}
	}
	return pool.Wait()
}

func (s *Site) RenderTag(tag string) error {
	// Render tag index.
	url, err := s.Config.TagURL(tag)
	if err != nil {
		return fmt.Errorf("cannot generate tag index %q: %w", tag, err)
	}
	p := NewTagIndex(tag, url)
	data, err := s.Layouts.RenderPage(p, DefaultTagIndexLayout)
	if err != nil {
		return err
	}
	log.Printf("T > %s\n", filepath.Join(OutDirName, p.Filename))
	// Apply filter.
	b, err := s.PageFilters.ApplyFilter(filepath.Ext(p.Filename), []byte(data))
	if err != nil {
		return err
	}
	if s.sitemap != nil {
		// Add to sitemap.
		if p.InSitemap() {
			if err := s.sitemap.Add(p.SitemapEntry()); err != nil {
				return err
			}
		}
	}
	// Write to file.
	return s.fileWriter.WriteFile(filepath.Join(s.BaseDir, OutDirName, p.Filename), b)

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
	fileExt := filepath.Ext(p.Filename)
	// Apply filter.
	b, err := s.PageFilters.ApplyFilter(fileExt, []byte(data))
	if err != nil {
		return err
	}
	if s.sitemap != nil {
		switch fileExt {
		case ".htm", ".html", ".xml":
			// Add to sitemap.
			if p.InSitemap() {
				if err := s.sitemap.Add(p.SitemapEntry()); err != nil {
					return err
				}
			}
		default:
			// nothing
		}
	}
	// Write to file.
	return s.fileWriter.WriteFile(filepath.Join(s.BaseDir, OutDirName, p.Filename), b)
}

func (s *Site) RenderPages() error {
	log.Printf("* Rendering pages")
	inDir := filepath.Join(s.BaseDir, PagesDirName)
	pool := utils.NewPool()
	err := filepath.Walk(inDir, func(path string, fi os.FileInfo, err error) error {
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
		if !pool.Add(func() error { return s.RenderPage(inDir, relname) }) {
			return filepath.SkipDir
		}
		return nil
	})
	if perr := pool.Wait(); perr != nil {
		return perr
	}
	return err
}

func (s *Site) CopyFile(filename string) error {
	inDir := filepath.Join(s.BaseDir, PagesDirName)
	outDir := filepath.Join(s.BaseDir, OutDirName)
	inFile := filepath.Join(inDir, filename)
	outFile := filepath.Join(outDir, filename)

	if err := s.fileWriter.CopyFile(outFile, inFile); err != nil {
		return err
	}
	log.Printf("C > %s\n", filepath.Join(OutDirName, filename))
	return nil
}

func (s *Site) RenderSitemap() error {
	if s.sitemap != nil {
		log.Printf("* Rendering sitemap.")
		var buf bytes.Buffer
		if err := s.sitemap.Render(&buf, s.Config.URL); err != nil {
			return err
		}
		return s.fileWriter.WriteFile(filepath.Join(OutDirName, s.Config.Sitemap), buf.Bytes())
	}
	return nil
}

func (s *Site) ProcessAssets() error {
	log.Printf("* Processing assets.")
	return s.Assets.Process()
}

func (s *Site) RenderAssets() error {
	log.Printf("* Rendering assets.")
	outDir := filepath.Join(s.BaseDir, OutDirName)
	if s.Config.Static != nil && s.Config.Static.Assets {
		outDir = filepath.Join(outDir, s.Config.Static.Path)
	}
	return s.Assets.Render(s.fileWriter, outDir)
}

func (s *Site) runBuild() error {
	if s.cleanBeforeBuilding {
		if err := s.Clean(); err != nil {
			return err
		}
	}
	// Reload config.
	if err := s.LoadConfig(); err != nil {
		return err
	}
	s.Config.Date = time.Now()

	markup.SetOptions(s.Config.Markup)

	if err := s.LoadPageFilters(); err != nil {
		return err
	}
	if err := s.LoadAssets(); err != nil {
		return err
	}
	if err := s.LoadCSP(); err != nil {
		return err
	}
	if err := s.LoadIncludes(); err != nil {
		return err
	}
	if err := s.LoadLayoutFuncs(); err != nil {
		return err
	}
	if err := s.LoadLayouts(); err != nil {
		return err
	}
	if err := s.LoadPosts(); err != nil {
		return err
	}
	if err := s.ProcessAssets(); err != nil {
		return err
	}
	if err := s.RenderAssets(); err != nil {
		return err
	}
	if err := s.RenderPosts(); err != nil {
		return err
	}
	if err := s.RenderPages(); err != nil {
		return err
	}
	if s.Config.TagIndex != nil {
		if err := s.RenderTagsIndex(); err != nil {
			return err
		}
	}
	if err := s.RenderSitemap(); err != nil {
		return err
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
	if s.Config.Search != nil {
		if err := s.generateSearchIndex(); err != nil {
			return err
		}
	}
	log.Printf("* Built in %s", time.Now().Sub(t))
	return nil
}

func (s *Site) isExcludedFromSearch(url string) bool {
	if s.Config.Search == nil {
		return false
	}
	for _, ex := range s.Config.Search.Exclude {
		if ex == url {
			return true
		}
	}
	return false
}

func (s *Site) generateSearchIndex() error {
	log.Printf("* Indexing")
	if s.Config.Search.Index == "" {
		log.Fatal("missing search.script config")
	}
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
		defer f.Close()
		url := utils.CleanPermalink(filepath.ToSlash(path[len(dir):]))
		if s.isExcludedFromSearch(url) {
			return nil
		}
		indexed, err := index.AddHTML(url, f)
		if err != nil {
			return err
		}
		if indexed {
			n++
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	if n == 0 {
		log.Println("* No documents indexed.")
		return nil
	}
	w := bytes.NewBuffer(nil)
	if err := index.WriteJSON(w); err != nil {
		return err
	}
	out := w.Bytes()
	filename := filepath.Join(s.BaseDir, OutDirName, s.Config.Search.Index)
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return err
	}
	if err := s.fileWriter.WriteFile(filename, out); err != nil {
		return err
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
	return s.layoutFuncs
}

func (s *Site) LoadLayoutFuncs() error {
	s.layoutFuncs = layouts.FuncMap{
		// `xml` function escapes XML.
		"xml": func(in string) (string, error) {
			var buf bytes.Buffer
			if err := xml.EscapeText(&buf, []byte(in)); err != nil {
				return "", err
			}
			return buf.String(), nil
		},
		// `json` function returns encoded JSON string (without quotes)
		"json": func(in string) (string, error) {
			var buf bytes.Buffer
			enc := json.NewEncoder(&buf)
			enc.SetEscapeHTML(false)
			err := enc.Encode(in)
			if err != nil {
				return "", err
			}
			out := buf.String()
			// slice out quotes and new line
			return out[1 : len(out)-2], nil
		},
		// `asset` function returns asset URL or content by its name.
		"asset": func(name string) (string, error) {
			a := s.Assets.Get(name)
			if a == nil {
				return "", fmt.Errorf("asset %q not found", name)
			}
			if a.IsBuffered() {
				return string(a.Result), nil
			}
			resultURL := a.RenderedName
			if s.Config.Static != nil && s.Config.Static.Assets {
				joined, err := url.JoinPath(s.Config.Static.URL, resultURL)
				if err != nil {
					return "", err
				}
				resultURL = joined
			}
			return resultURL, nil
		},
		// `static` function joins URL from site config's static.url with the given URL.
		"static": func(staticURL string) (string, error) {
			if s.Config.Static != nil {
				return url.JoinPath(s.Config.Static.URL, staticURL)
			} else {
				return path.Join("/", staticURL), nil
			}
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
		// `csp` returns Content-Security-Policy string.
		"csp": func() (string, error) {
			if len(s.CSP) == 0 {
				return "", errors.New("CSP is empty, check csp.yml")
			}
			return s.CSP.String(), nil
		},
		// `lastindex` returns the index of the last element of a slice.
		"lastindex": func(item reflect.Value) (int, error) {
			switch item.Kind() {
			case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
				return item.Len() - 1, nil
			}
			return 0, fmt.Errorf("lastindex of type %s", item.Type())
		},
	}
	return nil
}

func (s *Site) Serve(addr string) error {
	outDir := filepath.Join(s.BaseDir, OutDirName)
	log.Printf("Serving at %s. Press Ctrl+C to quit.\n", addr)
	return http.ListenAndServe(addr, http.FileServer(http.Dir(outDir)))
}

func (s *Site) StartWatching() (err error) {
	// Watch every subdirectory of site except for output directory and .git.
	excludeGlobs := []string{
		filepath.Join(s.BaseDir, OutDirName),
		filepath.Join(s.BaseDir, ".git"),
		".DS_Store",
	}
	watcher, err := fspoll.Watch(s.BaseDir, excludeGlobs, 0, 0)
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
				log.Printf("! watcher error: %s", err)
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
