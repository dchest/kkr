// Copyright (C) 2012 Dmitry Chestnykh <dmitry@codingrobots.com>
// License: GPL 3
package page

import (
	"time"
	"path/filepath"
	"html/template"
	"fmt"
	"os"
)

type Post struct {
	Page
	Date time.Time
}

func LoadPost(basedir, filename string) (p *Post, err error) {
	page, err := LoadPage(basedir, filename)
	if err != nil {
		return
	}
	if page.Template == defaultPageTemplate {
		page.Template = defaultPostTemplate
	}
	// Transform:
	// 	/path/to/2006-01-02-postname.html
	// to:
	// 	/path/to/2006/01/02/postname/index.html
	// and extract date.
	basefile := filepath.Base(filename)
	// Remove extensions.
	basefile = basefile[:len(basefile)-len(filepath.Ext(basefile))]
	if len(basefile) < len("2006-01-02-") {
		err = fmt.Errorf("wrong post filename format %q", basefile)
		return
	}
	date, err := time.Parse("2006-01-02", basefile[0:len("2006-01-02")])
	if err != nil {
		return
	}
	page.Filename = filepath.Join(
		filepath.Dir(filename), // /path/to/
		basefile[0:4],		// 2006
		basefile[5:7],		// 01
		basefile[8:10],		// 02
		basefile[11:],		// postname
		"index.html",		// index.html
	)
	return &Post{
		Page: *page,
		Date: date,
	}, nil
}


func (p *Post) Render(outdir string, templates *template.Template) error {
	tpl := templates.Lookup(p.Template)
	if tpl == nil {
		return fmt.Errorf("post: template %q not found", p.Template)
	}
	if err := os.MkdirAll(filepath.Join(outdir, filepath.Dir(p.Filename)), 0755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(outdir, p.Filename))
	if err != nil {
		return err
	}
	defer f.Close()
	return tpl.Execute(f, p)
}
