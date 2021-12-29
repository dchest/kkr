package search

import (
	_ "embed"
	"log"
	"strings"

	"github.com/dchest/jsmin"
	"github.com/dchest/kkr/search/indexer"
)

//go:embed ui/stemmer.min.js
var stemmer string

//go:embed ui/search.js
var mainScript string

func GetSearchScript(searchIndexURL string) string {
	script := strings.ReplaceAll(mainScript, "__KKR_SEARCH_INDEX_URL__", searchIndexURL)
	script = strings.ReplaceAll(script, "__KKR_STOP_WORDS__", indexer.StopWords)
	out := stemmer + script
	minified, err := jsmin.Minify([]byte(out))
	if err != nil {
		log.Printf("Failed to minify search-script, continuing with unminified")
	} else {
		out = string(minified)
	}
	return out
}
