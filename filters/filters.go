package filters

import (
	"fmt"
)

type Filter interface {
	Name() string
	Filter(string) (string, error)
}

type FilterMaker func([]string) Filter

var (
	filterMakersByName = make(map[string]FilterMaker)
	filtersByExt       = make(map[string]Filter)
)

func RegisterExt(ext, filterName string, args []string) error {
	fm, ok := filterMakersByName[filterName]
	if !ok {
		return fmt.Errorf("filter %q not found", filterName)
	}
	// Make sure ext starts with dot.
	if ext[0] != '.' {
		ext = "." + ext
	}
	filtersByExt[ext] = fm(args)
	return nil
}

func HasFilterForExt(ext string) bool {
	_, ok := filtersByExt[ext]
	return ok
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

func RegisterMaker(filterName string, fn func([]string) Filter) {
	filterMakersByName[filterName] = fn
}
