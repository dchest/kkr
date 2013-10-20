package assets

import (
	"crypto/md5"
	"encoding/base32"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/dchest/goyaml"

	"github.com/dchest/kkr/filters"
)

type Asset struct {
	Name    string   `yaml:"name"`
	Filter  []string `yaml:"filter,omitempty"`
	Files   []string `yaml:"files"`
	OutName string   `yaml:"outname"`

	Filename string
}

var assetsByName = make(map[string]*Asset)

func LoadAssets(filename string) error {
	// Read file.
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			// Have no assets, it's okay.
			return nil
		}
		return err
	}
	// Unmarshall YAML.
	var assets []*Asset
	err = goyaml.Unmarshal(b, &assets)
	if err != nil {
		return err
	}
	// Put assets into assetsByName map.
	for _, v := range assets {
		if _, exists := assetsByName[v.Name]; exists {
			return fmt.Errorf("duplicate asset name %q", v.Name)
		}
		assetsByName[v.Name] = v
	}
	return nil
}

func ProcessAssets(outdir string) error {
	for _, v := range assetsByName {
		// Load filters.
		if err := v.LoadFilter(); err != nil {
			return err
		}
		// Process.
		if err := v.Process(outdir); err != nil {
			return err
		}
	}
	return nil
}

// fillTemplate replaces ":hash" in template with hexadecimal characters of
// hash and returns the result.
func fillTemplate(template string, hash []byte) string {
	// 10 bytes of hash is enough to avoid accidental collisions.
	hs := strings.ToLower(base32.StdEncoding.EncodeToString(hash[:10]))
	return strings.Replace(template, ":hash", hs, -1)
}

func concatFiles(filenames []string) (out []byte, err error) {
	for _, f := range filenames {
		b, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, err
		}
		out = append(out, b...)
	}
	return out, nil
}

func (a *Asset) LoadFilter() error {
	if len(a.Filter) == 0 {
		return nil
	}
	return filters.RegisterAssetName(a.Name, a.Filter[0], a.Filter[1:])
}

func (a *Asset) Process(outdir string) error {
	// Concatenate files.
	b, err := concatFiles(a.Files)
	if err != nil {
		return err
	}
	// Filter result.
	if a.Filter != nil && filters.HasFilterForAssetName(a.Name) {
		s, _, err := filters.FilterTextByAssetName(a.Name, string(b))
		if err != nil {
			return err
		}
		b = []byte(s)

	}
	// Calculate hash.
	h := md5.New()
	h.Write(b)
	// Make name.
	a.Filename = fillTemplate(a.OutName, h.Sum(nil))
	// Check that the result is not empty.
	if a.Filename == "" {
		// Use hash for name.
		a.Filename = string(h.Sum(nil))
	}
	log.Printf("A %s", a.Filename)
	// Write to file.
	outfile := filepath.Join(outdir, filepath.FromSlash(a.Filename))
	if err := os.MkdirAll(filepath.Dir(outfile), 0755); err != nil {
		return err
	}
	return ioutil.WriteFile(outfile, b, 0644)
}

func AssetByName(name string) (*Asset, error) {
	a, ok := assetsByName[name]
	if !ok {
		return nil, fmt.Errorf("asset %q not found", name)
	}
	return a, nil
}

func Clean() {
	assetsByName = make(map[string]*Asset)
}
