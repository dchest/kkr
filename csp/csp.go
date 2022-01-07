// Package csp implements loading of Content-Security-Policy definitions
package csp

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/dchest/kkr/utils"
)

type Directives string

func (d Directives) String() string {
	return string(d)
}

// Load loads an CSP definition from the file and returns it.
func Load(filename string) (d Directives, err error) {
	m := make(map[string][]string)
	err = utils.UnmarshallYAMLFile(filename, &m)
	if err != nil {
		if os.IsNotExist(err) {
			// No assets file is not an error,
			// results in an empty directives map.
			err = nil
		} else {
			return
		}
	}
	return Directives(directivesToString(m)), nil
}

var quotableKeyword = regexp.MustCompile("^((none|self|unsafe-inline|unsafe-eval|strict-dynamic|unsafe-hashes|report-sample|unsafe-allow-redirects)|(nonce-.*|sha(256|384|512)-.*))$")

func quoteValues(a []string) []string {
	b := make([]string, len(a))
	for i, v := range a {
		if quotableKeyword.MatchString(v) {
			b[i] = "'" + v + "'"
		} else {
			b[i] = v
		}
	}
	return b
}

func directivesToString(m map[string][]string) string {
	keys := make([]string, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	out := make([]string, len(m))
	i = 0
	for _, k := range keys {
		out[i] = fmt.Sprintf("%s %s", k, strings.Join(quoteValues(m[k]), " "))
		i++
	}
	return strings.Join(out, ";")
}
