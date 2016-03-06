// Copyright 2013 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package assets implements loading, concatenating, and filtering of assets.
package assets

import (
	"encoding/base32"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

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
func (c *Collection) Process(outdir string) error {
	for _, a := range c.assets {
		if err := c.ProcessAsset(a, c.filters, outdir); err != nil {
			return err
		}
	}
	return nil
}

// Get returns an asset by name or nil if there's no such asset.
func (c *Collection) Get(name string) *Asset {
	return c.assets[name]
}

// fillTemplate replaces ":hash" in template with hexadecimal characters of
// hash and returns the result.
func fillTemplate(template string, hash []byte) string {
	// 10 bytes of hash is enough to avoid accidental collisions.
	hs := strings.ToLower(base32.StdEncoding.EncodeToString(hash[:10]))
	return strings.Replace(template, ":hash", hs, -1)
}

func concatFiles(filenames []string, separator string) (out []byte, err error) {
	sep := []byte(separator)
	for i, f := range filenames {
		if len(f) > 0 && f[0] == bufSigil {
			// Not a file, skip for now.
			continue
		}
		b, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, err
		}
		out = append(out, b...)
		if i != len(filenames)-1 {
			out = append(out, sep...)
		}
	}
	return out, nil
}

func (c *Collection) ProcessAsset(a *Asset, filters *filters.Collection, outdir string) error {
	// Concatenate files.
	b, err := concatFiles(a.Files, a.Separator)
	if err != nil {
		return err
	}
	// Append buffers if any.
	for _, f := range a.Files {
		if len(f) > 0 && f[0] == bufSigil {
			refAsset := c.Get(f[1:]) // e.g. $global-style -> global-style
			if refAsset == nil {
				return fmt.Errorf("asset %q not found", f[1:])
			}
			if !refAsset.processed {
				// Process it.
				// BUG Here hang if we can have a circular reference.
				if err := c.ProcessAsset(refAsset, filters, outdir); err != nil {
					return err
				}
			}
			b = append(b, []byte(refAsset.Result)...)
		}
	}
	// Filter result.
	s, err := filters.ApplyFilter(a.Name, string(b))
	if err != nil {
		return err
	}
	if a.OutName == string(bufSigil) {
		// Result is output.
		// Don't write to file, just remember result.
		a.Result = s
		a.processed = true
		log.Printf("A %c%s", bufSigil, a.Name)
		return nil
	}
	// Result is filename.
	// Make file name from hash.
	a.Result = fillTemplate(a.OutName, utils.Hash(s))
	// Check that the result is not empty.
	if a.Result == "" {
		// Use hash for name.
		a.Result = string(utils.Hash(s))
	}
	log.Printf("A %s", a.Result)
	// Write to file.
	outfile := filepath.Join(outdir, filepath.FromSlash(a.Result))
	if err := utils.WriteStringToFile(outfile, s); err != nil {
		return err
	}
	a.processed = true
	return nil
}
