package search

import (
	_ "embed"
	"log"
	"strings"

	"github.com/dchest/jsmin"
)

//go:embed ui/stemmer.min.js
var stemmer string

//go:embed ui/search.js
var mainScript string

func GetSearchScript(searchIndexURL string) string {
	out := stemmer + strings.ReplaceAll(mainScript, "__KKR_SEARCH_INDEX_URL__", searchIndexURL)
	minified, err := jsmin.Minify([]byte(out))
	if err != nil {
		log.Printf("Failed to minify search-script, continuing with unminified")
	} else {
		out = string(minified)
	}
	return out
}
