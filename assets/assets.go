package assets

import (
	"encoding/base32"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dchest/kkr/filters"
	"github.com/dchest/kkr/utils"
)

type Asset struct {
	Name      string      `yaml:"name"`
	Filter    interface{} `yaml:"filter,omitempty"`
	Files     []string    `yaml:"files"`
	Separator string      `yaml:"separator,omitempty"`
	OutName   string      `yaml:"outname"`

	Filename string
}

type Collection struct {
	sync.Mutex
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
	c.Lock()
	defer c.Unlock()
	for _, v := range c.assets {
		if err := v.Process(c.filters, outdir); err != nil {
			return err
		}
	}
	return nil
}

// Get returns an asset by name or nil if there's no such asset.
func (c *Collection) Get(name string) *Asset {
	c.Lock()
	defer c.Unlock()
	return c.assets[name]
}

// fillTemplate replaces ":hash" in template with hexadecimal characters of
// hash and returns the result.
func fillTemplate(template string, hash []byte) string {
	// 10 bytes of hash is enough to avoid accidental collisions.
	hs := strings.ToLower(base32.StdEncoding.EncodeToString(hash[:10]))
	return strings.Replace(template, ":hash", hs, -1)
}

func concatFiles(filenames []string,  separator string) (out []byte, err error) {
	sep := []byte(separator)
	for i, f := range filenames {
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

func (a *Asset) Process(filters *filters.Collection, outdir string) error {
	// Concatenate files.
	b, err := concatFiles(a.Files, a.Separator)
	if err != nil {
		return err
	}
	// Filter result.
	s, err := filters.ApplyFilter(a.Name, string(b))
	if err != nil {
		return err
	}
	// Make name from hash.
	a.Filename = fillTemplate(a.OutName, utils.Hash(s))
	// Check that the result is not empty.
	if a.Filename == "" {
		// Use hash for name.
		a.Filename = string(utils.Hash(s))
	}
	log.Printf("A %s", a.Filename)
	// Write to file.
	outfile := filepath.Join(outdir, filepath.FromSlash(a.Filename))
	return utils.WriteStringToFile(outfile, s)
}
