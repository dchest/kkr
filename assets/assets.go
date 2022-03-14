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

	// Result is output filename, if OutName is "$", the content of output.
	Result string

	processed bool
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
func (c *Collection) Process(fw *filewriter.FileWriter, outdir string) error {
	for _, a := range c.assets {
		if err := c.ProcessAsset(fw, a, c.filters, outdir); err != nil {
			return err
		}
	}
	return nil
}

func (c *Collection) SetStringAsset(name, data string) {
	c.assets[name] = &Asset{
		Name:      name,
		OutName:   "$",
		Result:    data,
		processed: true,
	}
}

// Get returns an asset by name or nil if there's no such asset.
func (c *Collection) Get(name string) *Asset {
	return c.assets[name]
}

func isBufferName(s string) bool {
	return len(s) > 0 && s[0] == bufSigil
}

func (c *Collection) ProcessAsset(fw *filewriter.FileWriter, a *Asset, filters *filters.Collection, outdir string) error {
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
				if err := c.ProcessAsset(fw, refAsset, filters, outdir); err != nil {
					return err
				}
			}
			buf.WriteString(refAsset.Result)
		} else {
			f, err := os.Open(name)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(&buf, f); err != nil {
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
	if a.OutName == string(bufSigil) {
		// Result is the output.
		// Don't write to file, just remember the result.
		a.Result = string(b)
		a.processed = true
		log.Printf("A %c%s", bufSigil, a.Name)
		return nil
	}
	// Result is filename.
	// Make file name from hash.
	a.Result = utils.TemplatedHash(a.OutName, b)
	// Check that the result is not empty.
	if a.Result == "" {
		return fmt.Errorf("templated hash for asset %s returned empty result", a.Name)
	}
	log.Printf("A %s", a.Result)
	// Write to file.
	outfile := filepath.Join(outdir, filepath.FromSlash(a.Result))
	if err := fw.WriteFile(outfile, b); err != nil {
		return err
	}
	a.processed = true
	return nil
}
