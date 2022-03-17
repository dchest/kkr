// Copyright 2013 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package assets implements loading, concatenating, and filtering of assets.
package assets

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/dchest/kkr/filewriter"
	"github.com/dchest/kkr/filters"
	"github.com/dchest/kkr/utils"
)

const bufSigil = '$'

type Asset struct {
	Name      string      `yaml:"name"`
	Filter    interface{} `yaml:"filter,omitempty"`
	Files     []string    `yaml:"files"`
	Separator string      `yaml:"separator,omitempty"`
	OutName   string      `yaml:"outname"`

	// RenderedName is the output filename of the asset,
	// or an empty string if OutName is "$".
	RenderedName string

	// Result is the processed content of asset.
	Result []byte

	processed bool
}

// IsBuffered returns true if the output of asset
// is a buffer (OutName starts with $).
func (a *Asset) IsBuffered() bool {
	return isBufferName(a.OutName)
}

type Collection struct {
	assets  map[string]*Asset
	filters *filters.Collection
}

// Load loads an asset collection from the given assets config file and returns it.
func Load(filename string) (c *Collection, err error) {
	// Load assets description from file (or create a new).
	var assets []*Asset
	err = utils.UnmarshallYAMLFile(filename, &assets)
	if err != nil {
		if os.IsNotExist(err) {
			// No assets file is not an error,
			// create an empty collection.
			assets = make([]*Asset, 0)
			err = nil
		} else {
			return
		}
	}

	// Put assets into a map addressed by name and load filters.
	c = &Collection{
		assets:  make(map[string]*Asset),
		filters: filters.NewCollection(),
	}
	for _, v := range assets {
		if _, exists := c.assets[v.Name]; exists {
			return nil, fmt.Errorf("duplicate asset name %q", v.Name)
		}
		c.assets[v.Name] = v
		if v.Filter != nil {
			c.filters.AddFromYAML(v.Name, v.Filter)
		}
	}
	return c, nil
}

// Process processes all assets in the collection.
func (c *Collection) Process() error {
	for _, a := range c.assets {
		if err := c.ProcessAsset(a, c.filters); err != nil {
			return err
		}
	}
	return nil
}

func (c *Collection) Render(fw *filewriter.FileWriter, outdir string) error {
	for _, a := range c.assets {
		if a.IsBuffered() {
			continue
		}
		if err := c.RenderAsset(a, fw, outdir); err != nil {
			return err
		}
	}
	return nil
}

func (c *Collection) SetStringAsset(name, data string) {
	c.assets[name] = &Asset{
		Name:         name,
		OutName:      "$",
		RenderedName: "",
		Result:       []byte(data),
		processed:    true,
	}
}

// Get returns an asset by name or nil if there's no such asset.
func (c *Collection) Get(name string) *Asset {
	return c.assets[name]
}

func isBufferName(s string) bool {
	return len(s) > 0 && s[0] == bufSigil
}

func readFile(w io.Writer, filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(w, f); err != nil {
		return err
	}
	return nil
}

func (c *Collection) ProcessAsset(a *Asset, filters *filters.Collection) error {
	if a.processed {
		return nil
	}
	separator := a.Separator
	// Concatenate files and buffers.
	var buf bytes.Buffer
	for i, name := range a.Files {
		if isBufferName(name) {
			refAsset := c.Get(name[1:]) // e.g. $global-style -> global-style
			if refAsset == nil {
				return fmt.Errorf("asset %q not found", name[1:])
			}
			if !refAsset.processed {
				// Process it.
				// BUG Here will hang if we can have a circular reference.
				if err := c.ProcessAsset(refAsset, filters); err != nil {
					return err
				}
			}
			buf.Write(refAsset.Result)
		} else {
			if err := readFile(&buf, name); err != nil {
				return err
			}
		}
		if i != len(a.Files)-1 {
			buf.WriteString(separator)
		}
	}
	// Filter result.
	b, err := filters.ApplyFilter(a.Name, buf.Bytes())
	if err != nil {
		return err
	}
	a.Result = b
	if a.IsBuffered() {
		a.RenderedName = ""
	} else {
		a.RenderedName = utils.TemplatedHash(a.OutName, b)
		if a.RenderedName == "" {
			return fmt.Errorf("templated hash for asset %s returned empty result", a.Name)
		}
	}
	a.processed = true
	return nil
}

func (c Collection) RenderAsset(a *Asset, fw *filewriter.FileWriter, outdir string) error {
	if a.IsBuffered() {
		return nil // this asset shouldn't be rendered into a file
	}
	log.Printf("A %s", a.RenderedName)
	outfile := filepath.Join(outdir, filepath.FromSlash(a.RenderedName))
	return fw.WriteFile(outfile, a.Result)
}
