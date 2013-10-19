package filters

import (
	"fmt"
)

type Filter interface {
	Name() string
	Filter(string) (string, error)
}

var (
	filtersByName = make(map[string]Filter)
	filtersByExt  = make(map[string]Filter)
)

func Register(f Filter) {
	filtersByName[f.Name()] = f
}

func FilterText(filterName string, text string) (out string, err error) {
	f, ok := filtersByName[filterName]
	if !ok {
		return text, fmt.Errorf("filter %q not found", filterName)
	}
	return f.Filter(text)
}

func RegisterExt(ext, filterName string) error {
	f, ok := filtersByName[filterName]
	if !ok {
		return fmt.Errorf("filter %q not found", filterName)
	}
	// Make sure ext starts with dot.
	if ext[0] != '.' {
		ext = "." + ext
	}
	filtersByExt[ext] = f
	return nil
}

func FilterTextByExt(ext string, text string) (out string, filterName string, err error) {
	f, ok := filtersByExt[ext]
	if !ok {
		// No filter.
		return text, "", nil
	}
	filterName = f.Name()
	out, err = f.Filter(text)
	return
}

type filterFromFunc struct {
	name   string
	filter func(string) (string, error)
}

func (f *filterFromFunc) Name() string                    { return f.name }
func (f *filterFromFunc) Filter(s string) (string, error) { return f.filter(s) }

func RegisterFunc(name string, fn func(string) (string, error)) {
	Register(&filterFromFunc{name, fn})
}
