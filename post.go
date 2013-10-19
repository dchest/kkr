// Copyright (C) 2012 Dmitry Chestnykh <dmitry@codingrobots.com>
// License: GPL 3
package main

import (
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Post struct {
	Page
	Date time.Time
}

func LoadPost(basedir, filename, outNameTemplate string) (p *Post, err error) {
	page, err := LoadPage(basedir, filename)
	if err != nil {
		return
	}
	// Extract date from:
	// 	/path/to/2006-01-02-postname.html
	basefile := path.Base(filename)
	// Remove extensions.
	basefile = basefile[:len(basefile)-len(path.Ext(basefile))]
	if len(basefile) < len("2006-01-02-") {
		err = fmt.Errorf("wrong post filename format %q", basefile)
		return
	}
	date, err := time.Parse("2006-01-02", basefile[0:len("2006-01-02")])
	if err != nil {
		return
	}

	// Fill out name template.
	replacements := map[string]string{
		":year":  basefile[0:4],
		":month": basefile[5:7],
		":day":   basefile[8:10],
		":name":  basefile[11:],
	}
	outname := outNameTemplate
	for k, v := range replacements {
		outname = strings.Replace(outname, k, v, -1)
	}

	url := cleanPermalink(outname)
	// Add properies to meta
	page.Meta["date"] = date
	page.Meta["url"] = url
	page.Meta["id"] = basefile

	// Add index.html if ends with slash.
	if outname[len(outname)-1] == '/' {
		outname += "index.html"
	}
	page.Filename = filepath.FromSlash(outname)
	page.URL = url
	return &Post{
		Page: *page,
		Date: date,
	}, nil
}

type Posts []*Post

func (pp Posts) Limit(n int) Posts {
	if n > len(pp) {
		n = len(pp)
	}
	return pp[:n]
}

func (pp Posts) From(n int) Posts {
	return pp[n:]
}

func (pp Posts) Len() int           { return len(pp) }
func (pp Posts) Less(i, j int) bool { return pp[i].Date.Before(pp[j].Date) }
func (pp Posts) Swap(i, j int)      { pp[i], pp[j] = pp[j], pp[i] }

func (pp Posts) Sort() {
	sort.Sort(pp)
}
