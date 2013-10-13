// Copyright (C) 2012 Dmitry Chestnykh <dmitry@codingrobots.com>
// License: GPL 3
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"

	"github.com/dchest/goyaml"
	"github.com/dchest/blackfriday"
)

const (
	propSep = "---\n"
	defaultPageTemplate = "default.html"
	defaultPostTemplate = "post.html"
)

type Page struct {
	Props map[string]interface{}
	Template string
	Content template.HTML
	Basedir string
	Filename string
}

func LoadPage(basedir, filename string) (p *Page, err error) {
	f, err := os.Open(filepath.Join(basedir, filename))
	if err != nil {
		return
	}
	defer f.Close()
	// Read properties.
	b := bufio.NewReader(f)
	head, err := b.ReadString('\n')
	if err != nil {
		return
	}
	if head != propSep {
		err = fmt.Errorf("%q: pages should start with %s (got %q)", filename, propSep, head)
		return
	}
	ybuf := bytes.NewBuffer(nil)
	for {
		var s string
		s, err = b.ReadString('\n')
		if err != nil {
			return
		}
		if s == propSep {
			break
		}
		ybuf.WriteString(s)
	}
	props := make(map[string]interface{})
	err = goyaml.Unmarshal(ybuf.Bytes(), &props)
	if err != nil {
		return
	}
	templateName := defaultPageTemplate
	if tpl, ok := props["template"]; ok {
		if v, ok := tpl.(string); ok {
			templateName = v
		}
	}
	// Read the rest of file.
	cbuf := bytes.NewBuffer(nil)
	_, err = io.Copy(cbuf, b)
	if err != nil {
		return
	}
	var content string
	if markup, ok := props["markup"]; ok {
		if markup != "markdown" {
			err = fmt.Errorf("unknown markup: %q", markup)
		}
		content = string(blackfriday.MarkdownCommon(cbuf.Bytes()))
	} else {
		content = cbuf.String()
	}
	return &Page {
		Props: props,
		Template: templateName,
		Basedir: basedir,
		Filename: filename,
		Content: template.HTML(content),
	}, nil
}

func (p *Page) Render(outdir string, templates *template.Template) error {
	tpl := templates.Lookup(p.Template)
	if tpl == nil {
		return fmt.Errorf("page: template %q not found", p.Template)
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
