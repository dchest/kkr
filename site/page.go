// Copyright 2013 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package site

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dchest/kkr/markup"
	"github.com/dchest/kkr/metafile"
	"github.com/dchest/kkr/sitemap"
	"github.com/dchest/kkr/utils"
)

type cache struct {
	mu sync.Mutex
	m  map[string]*Page
}

func (c *cache) Get(name string) *Page {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.m[name]
}

func (c *cache) Put(name string, page *Page) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[name] = page
}

var pageCache *cache

func EnableCache(value bool) {
	if value {
		pageCache = &cache{
			m: make(map[string]*Page),
		}
	} else {
		pageCache = nil
	}
}

type Page struct {
	fi           os.FileInfo
	uid          string
	meta         map[string]interface{}
	content      string
	ShortContent string // content before <!--more-->, or empty if none
	Basedir      string
	Filename     string
	url          string
}

func (p *Page) Meta() map[string]interface{} { return p.meta }
func (p *Page) Content() string              { return p.content }
func (p *Page) FileInfo() os.FileInfo        { return p.fi }
func (p *Page) URL() string                  { return p.url }

func (p *Page) InSitemap() bool {
	if value, ok := p.meta["sitemap"].(bool); ok {
		return value
	}
	return true
}

func (p *Page) SitemapEntry() sitemap.Entry {
	changefreq := ""
	if mchangefreq, ok := p.meta["changefreq"]; ok {
		changefreq = fmt.Sprintf("%v", mchangefreq)
	}
	priority := ""
	if mpriority, ok := p.meta["priority"]; ok {
		priority = fmt.Sprintf("%v", mpriority)
	}
	lastmod := ""
	if p.fi != nil {
		lastmod = p.fi.ModTime().Format("2006-01-02")
	}
	return sitemap.Entry{
		Loc:        p.url,
		Lastmod:    lastmod,
		Changefreq: changefreq,
		Priority:   priority,
	}
}

var NotPageError = errors.New("not a page or post")

func IsNotPage(err error) bool {
	return err == NotPageError
}

const moreSeparator = "<!--more-->"

func extractShortContent(s string) (shortContent, content string) {
	i := strings.Index(s, moreSeparator)
	if i < 0 {
		return "", s
	}
	shortContent = s[:i]
	content = s[:i] + `<a name="more"></a>` + s[i+len(moreSeparator):]
	return
}

func LoadPage(basedir, filename string) (p *Page, err error) {
	fullname := filepath.Join(basedir, filename)
	if pageCache != nil {
		// Try getting from cache
		page := pageCache.Get(fullname)
		if page != nil && !metafile.Changed(fullname, page.fi) {
			return page, nil
		}
	}
	f, err := metafile.Open(fullname)
	if err != nil {
		return
	}
	defer f.Close()

	if !f.HasMeta() {
		return nil, NotPageError
	}

	meta := f.Meta()
	content, err := f.Content()
	if err != nil {
		return
	}

	// If page is a Markdown file, set its markup meta to Markdown (to
	// process content) and replace output file extension with .html.
	if utils.HasFileExt(filename, MarkdownExtensions) {
		meta["markup"] = "markdown"
		filename = utils.ReplaceFileExt(filename, ".html")
	}

	if markupName, ok := meta["markup"]; ok {
		markupName, ok := markupName.(string)
		if !ok {
			return nil, errors.New("markup must be a string")
		}
		content, err = markup.Process(markupName, content)
		if err != nil {
			return
		}
	}

	// Change filename if there's 'permalink'.
	if permalink, ok := meta["permalink"]; ok {
		filename = filepath.FromSlash(permalink.(string))
	}

	// Change filename to filename/index.html
	// if 'folder' is true.
	if folder, ok := meta["folder"]; ok && folder.(bool) {
		filename = filepath.Join(utils.ReplaceFileExt(filename, ""), "index.html")
	}

	url := utils.CleanPermalink(filepath.ToSlash(filename))
	meta["url"] = url
	meta["id"] = filepath.ToSlash(filename)

	shortContent, contentStr := extractShortContent(string(content))

	p = &Page{
		fi:           f.FileInfo(),
		meta:         meta,
		ShortContent: shortContent,
		content:      contentStr,
		Basedir:      basedir,
		Filename:     filename,
		url:          url,
	}
	if pageCache != nil {
		// Cache this page
		pageCache.Put(fullname, p)
	}
	return p, nil
}
