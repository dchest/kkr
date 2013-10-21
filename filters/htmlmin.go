package filters

import (
	"github.com/dchest/htmlmin"
)

// `htmlmin` is a primitive not-so-correct HTML minimizer filter.

func init() {
	Register("htmlmin", func(args []string) Filter {
		return HTMLMin(0)
	})
}

type HTMLMin int

func (f HTMLMin) Name() string { return "htmlmin" }

func (f HTMLMin) Apply(s string) (out string, err error) {
	result, err := htmlmin.Minify([]byte(s))
	if err != nil {
		return "", err
	}
	return string(result), nil
}
