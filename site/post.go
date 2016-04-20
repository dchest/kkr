// Copyright 2013 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package site

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dchest/kkr/utils"
)

type Post struct {
	Page
	Tags []string
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
	// Now try getting date from meta.
	if md, ok := page.meta["date"]; ok {
		switch d := md.(type) {
		case string:
			date, err = utils.ParseAnyDate(d)
			if err != nil {
				return nil, err
			}
		case time.Time:
			// already processed, do nothing
		default:
			return nil, errors.New("'date' is not a string")
		}
	}

	// Fill out name template.
	replacements := []struct{ template, rep string }{
		{":year", basefile[0:4]},
		{":month", basefile[5:7]},
		{":day", basefile[8:10]},
		{":name", basefile[11:]},
	}
	outname := outNameTemplate
	for _, v := range replacements {
		outname = strings.Replace(outname, v.template, v.rep, -1)
	}

	url := utils.CleanPermalink(outname)
	// Add properies to meta
	page.meta["date"] = date
	page.meta["url"] = url
	page.meta["id"] = basefile
	page.meta["is_post"] = true

	// Get tags.
	var tags []string
	if mt, ok := page.meta["tags"]; ok {
		switch t := mt.(type) {
		case string:
			tags = strings.Split(t, ",")
			for i, v := range tags {
				tags[i] = strings.TrimSpace(v)
			}
		case []interface{}:
			tags = make([]string, 0, len(t))
			for _, v := range t {
				tags = append(tags, v.(string))
			}
		default:
			return nil, errors.New("'tags' is not an array of strings or a string")
		}
	}

	// Add index.html if ends with slash.
	if outname[len(outname)-1] == '/' {
		outname += "index.html"
	}
	page.Filename = filepath.FromSlash(outname)
	page.URL = url
	return &Post{
		Page: *page,
		Date: date,
		Tags: tags,
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
func (pp Posts) Less(i, j int) bool { return pp[i].Date.After(pp[j].Date) }
func (pp Posts) Swap(i, j int)      { pp[i], pp[j] = pp[j], pp[i] }

func (pp Posts) Sort() {
	sort.Sort(pp)
}
