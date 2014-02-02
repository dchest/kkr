// Copyright 2013 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package site

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/dchest/blackfriday"

	"github.com/dchest/kkr/metafile"
	"github.com/dchest/kkr/utils"
)

type Page struct {
	meta     map[string]interface{}
	content  string
	Basedir  string
	Filename string
	URL      string
}

func (p *Page) Meta() map[string]interface{} { return p.meta }
func (p *Page) Content() string              { return p.content }

var NotPageError = errors.New("not a page or post")

func IsNotPage(err error) bool {
	return err == NotPageError
}

func LoadPage(basedir, filename string) (p *Page, err error) {
	f, err := metafile.Open(filepath.Join(basedir, filename))
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

	if markup, ok := meta["markup"]; ok {
		if markup != "markdown" {
			err = fmt.Errorf("unknown markup: %q", markup)
			return
		}
		content = blackfriday.MarkdownCommon(content)
	}

	// Change filename if there's 'permalink'.
	if permalink, ok := meta["permalink"]; ok {
		filename = filepath.FromSlash(permalink.(string))
	}

	url := utils.CleanPermalink(filepath.ToSlash(filename))
	meta["url"] = url
	meta["id"] = filepath.ToSlash(filename)

	return &Page{
		meta:     meta,
		content:  string(content),
		Basedir:  basedir,
		Filename: filename,
		URL:      url,
	}, nil
}
