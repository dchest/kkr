package layout

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"github.com/dchest/kkr/metafile"
	"github.com/dchest/kkr/assets"
)

type Layout struct {
	name   string
	parent string
	meta   map[string]interface{}
	tpl    *template.Template
}

var layouts = make(map[string]*Layout)

// Functions used in templates.
var funcMap = template.FuncMap{
	// xml escapes XML.
	"xml": func(s string) (string, error) {
		var buf bytes.Buffer
		if err := xml.EscapeText(&buf, []byte(s)); err != nil {
			return "", err
		}
		return buf.String(), nil
	},
	// asset returns asset filename by its name.
	"asset": func(name string) (string, error) {
		a, err := assets.AssetByName(name)
		if err != nil {
			return "", err
		}
		return a.Filename, nil
	},
}

func New(name string, defaultParent string, meta map[string]interface{}, content string) (l *Layout, err error) {
	l = new(Layout)
	l.name = name
	l.meta = meta
	parent, ok := meta["layout"]
	if ok {
		switch s := parent.(type) {
		case string:
			l.parent = s
		default:
			return nil, errors.New("'layout' must be a string")
		}
	} else {
		l.parent = defaultParent
	}
	if l.parent == "none" {
		l.parent = ""
	}
	l.tpl, err = template.New(name).Funcs(funcMap).Parse(content)
	if err != nil {
		return nil, err
	}
	return l, nil
}

func readLayout(filename string) (*Layout, error) {
	f, err := metafile.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	basename := filepath.Base(filename)
	name := basename[:len(basename)-len(filepath.Ext(basename))]
	content, err := f.Content()
	if err != nil {
		return nil, err
	}
	return New(name, "", f.Meta(), string(content))
}

func AddFile(filename string) error {
	l, err := readLayout(filename)
	if err != nil {
		return err
	}
	layouts[l.name] = l
	log.Printf("L %s", l.name)
	return nil
}

func AddDir(dirname string) error {
	return filepath.Walk(dirname, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		return AddFile(path)
	})
}

func (l *Layout) Render(site, page interface{}, content string) (out string, err error) {
	// Execute current layout.
	var buf bytes.Buffer
	err = l.tpl.Execute(&buf, struct {
		Site, Page interface{}
		Content    string
	}{site, page, content})
	if err != nil {
		return "", err
	}

	out = buf.String()

	if l.parent != "" {
		// Execute parent layout on output.
		parentLayout, ok := layouts[l.parent]
		if !ok {
			return "", fmt.Errorf("layout %q not found", l.parent)
		}
		return parentLayout.Render(site, page, out)
	}

	return out, nil
}
