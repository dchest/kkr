// Copyright (C) 2012 Dmitry Chestnykh <dmitry@codingrobots.com>
// License: GPL 3
package main

import (
	"fmt"
	"html/template"
	"log"
	"os"
	"io"
	"path/filepath"

	"github.com/dchest/kukuruz/page"
)

const (
	templatesDirName = "_templates"
	pagesDirName = "_pages"
	postsDirName = "_posts"
	outDirName = "_out"
	postsOutDirName = "blog"
)

var pageExtentions = []string{".html", ".htm"}

func isPage(filename string) bool {
	ext := filepath.Ext(filename)
	for _, v := range(pageExtentions) {
		if v == ext {
			return true
		}
	}
	return false
}

var templates *template.Template

func loadTemplates(basedir string) (err error) {
	templates, err = template.ParseGlob(filepath.Join(basedir, templatesDirName, "*"))
	return
}

func copyFile(basedir string, filename string) error {
	indir := filepath.Join(basedir, pagesDirName)
	outdir := filepath.Join(basedir, outDirName)
	if err := os.MkdirAll(filepath.Join(outdir, filepath.Dir(filename)), 0755); err != nil {
		return err
	}
	in, err := os.Open(filepath.Join(indir, filename))
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(filepath.Join(outdir, filename))
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return nil
}

func renderPages(basedir string) error {
	indir := filepath.Join(basedir, pagesDirName)
	outdir := filepath.Join(basedir, outDirName)
	return filepath.Walk(indir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relname, err := filepath.Rel(indir, path)
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil // TODO(dchest): create directories?
		}
		if relname[len(relname)-1] == '~' {
			return nil // skip temporary files
		}
		if isPage(relname) {
			// Render templated page.
			p, err := page.LoadPage(indir, relname)
			if err != nil {
				return err
			}
			fmt.Printf("P %s → %s\n", relname, filepath.Join(outdir, p.Filename))
			if err := p.Render(outdir, templates); err != nil {
				return err
			}
		} else {
			fmt.Printf("C %s → %s\n", relname, filepath.Join(outdir, relname))
			if err := copyFile(basedir, relname); err != nil {
				return err
			}
		}
		return nil
	})
}

func renderPosts(basedir string) error {
	indir := filepath.Join(basedir, postsDirName)
	outdir := filepath.Join(basedir, outDirName, postsOutDirName)
	return filepath.Walk(indir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relname, err := filepath.Rel(indir, path)
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil // TODO(dchest): create directories?
		}
		if relname[len(relname)-1] == '~' {
			return nil // skip temporary files
		}
		if !isPage(relname) {
			return nil
		}
		// Render templated page.
		p, err := page.LoadPost(indir, relname)
		if err != nil {
			return err
		}
		fmt.Printf("B %s → %s\n", relname, filepath.Join(outdir, p.Filename))
		if err := p.Render(outdir, templates); err != nil {
			return err
		}
		return nil
	})
}

func isDirExist(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
}

func main() {
	log.SetFlags(0);
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("os.Getwd(): %s", err)
	}
	if err := loadTemplates(wd); err != nil {
		log.Fatalf("cannot load templates: %s", err)
	}

	if isDirExist(filepath.Join(wd, pagesDirName)) {
		if err := renderPages(wd); err != nil {
			log.Fatalf("cannot render page: %s", err)
		}
	} else {
		log.Println("No pages to render.")
	}

	if isDirExist(filepath.Join(wd, postsDirName)) {
		if err := renderPosts(wd); err != nil {
			log.Fatalf("cannot render post: %s", err)
		}
	} else {
		log.Println("No posts to render.")
	}
}
