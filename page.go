// Copyright (C) 2012 Dmitry Chestnykh <dmitry@codingrobots.com>
package main

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/dchest/blackfriday"

	"github.com/dchest/kkr/metafile"
)

type Page struct {
	Meta     map[string]interface{}
	Content  string
	Basedir  string
	Filename string
	URL      string
}

var NotPageError = errors.New("not a page")

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

	if markup, ok := meta["markup"]; ok {
		if markup != "markdown" {
			err = fmt.Errorf("unknown markup: %q", markup)
		}
		content = blackfriday.MarkdownCommon(content)
	}

	url := cleanPermalink(filepath.ToSlash(filename))
	meta["url"] = url
	meta["id"] = filepath.ToSlash(filename)

	return &Page{
		Meta:     meta,
		Basedir:  basedir,
		Filename: filename,
		URL:      url,
		Content:  string(content),
	}, nil
}
