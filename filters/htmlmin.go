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
			case "unquote":
				f.unquote = true
			}
		}
		return f
	})
}

type HTMLMin struct {
	scripts, styles, unquote bool
}

func (f *HTMLMin) Name() string {
	return fmt.Sprintf("htmlmin (scripts=%v styles=%v)", f.scripts, f.styles)
}

func (f *HTMLMin) Apply(in []byte) (out []byte, err error) {
	return htmlmin.Minify(in, &htmlmin.Options{
		MinifyScripts: f.scripts,
		MinifyStyles:  f.styles,
		UnquoteAttrs:  f.unquote,
	})
}
