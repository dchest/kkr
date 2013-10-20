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
	filtersEnabled = true

	filterMakersByName = make(map[string]FilterMaker)

	filtersByExt       = make(map[string]Filter)
	filtersByAssetName = make(map[string]Filter)
)

func RegisterExt(extension, filterName string, args []string) error {
	fm, ok := filterMakersByName[filterName]
	if !ok {
		return fmt.Errorf("filter %q not found", filterName)
	}
	// Make sure extension starts with dot.
	if extension[0] != '.' {
		extension = "." + extension
	}
	filtersByExt[extension] = fm(args)
	return nil
}

func RegisterAssetName(assetName string, filterName string, args []string) error {
	fm, ok := filterMakersByName[filterName]
	if !ok {
		return fmt.Errorf("filter %q not found", filterName)
	}
	filtersByAssetName[assetName] = fm(args)
	return nil
}

func HasFilterForExt(extension string) bool {
	_, ok := filtersByExt[extension]
	return ok
}

func HasFilterForAssetName(assetName string) bool {
	_, ok := filtersByAssetName[assetName]
	return ok
}

func FilterTextByExt(extension string, text string) (out string, filterName string, err error) {
	if !filtersEnabled {
		return text, "", nil
	}
	f, ok := filtersByExt[extension]
	if !ok {
		// No filter.
		return text, "", nil
	}
	filterName = f.Name()
	out, err = f.Filter(text)
	return
}

func FilterTextByAssetName(assetName string, text string) (out string, filterName string, err error) {
	if !filtersEnabled {
		return text, "", nil
	}
	f, ok := filtersByAssetName[assetName]
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

func SetEnabled(enabled bool) {
	filtersEnabled = enabled
}
