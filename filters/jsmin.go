// Copyright 2013 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package filters

// `htmljsmin` is htmlmin which also minifies inline scripts with jsmin.

import (
	"github.com/dchest/htmlmin"
)

func init() {
	Register("htmljsmin", func(args []string) Filter {
		return HTMLJSMin(0)
	})
}

type HTMLJSMin int

func (f HTMLJSMin) Name() string { return "htmljsmin" }

func (f HTMLJSMin) Apply(s string) (out string, err error) {
	result, err := htmlmin.Minify([]byte(s), &htmlmin.Options{MinifyScripts: true})
	if err != nil {
		return "", err
	}
	return string(result), nil
}
