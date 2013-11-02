package filters

// `htmlmin` minifies HTML and, optionally, embedded scripts and styles.
//
// Optional arguments:
//
// 'scripts', 'styles'.
//
// Usage examples:
//
//  htmlmin
//  - minifies just HTML
//
//  [htmlmin, scripts, styles]
//  - minifies HTML, embedded JavaScripts, and embedded and inline styles.

import (
	"fmt"
	"strings"

	"github.com/dchest/htmlmin"
)

func init() {
	Register("htmlmin", func(args []string) Filter {
		f := new(HTMLMin)
		for _, v := range args {
			switch strings.ToLower(v) {
			case "scripts", "js":
				f.scripts = true
			case "styles", "css":
				f.styles = true
			}
		}
		return f
	})
}

type HTMLMin struct {
	scripts bool
	styles  bool
}

func (f *HTMLMin) Name() string {
	return fmt.Sprintf("htmlmin (scripts=%v styles=%v)", f.scripts, f.styles)
}

func (f *HTMLMin) Apply(s string) (out string, err error) {
	result, err := htmlmin.Minify([]byte(s), &htmlmin.Options{
		MinifyScripts: f.scripts,
		MinifyStyles:  f.styles,
	})
	if err != nil {
		return "", err
	}
	return string(result), nil
}
