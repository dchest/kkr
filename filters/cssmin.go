// Copyright 2013 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package filters

// `cssmin` minifies CSS.

import (
	"github.com/dchest/cssmin"
)

func init() {
	Register("cssmin", func(args []string) Filter {
		return CSSMin(0)
	})
}

type CSSMin int

func (f CSSMin) Name() string { return "cssmin" }

func (f CSSMin) Apply(in []byte) (out []byte, err error) {
	return cssmin.Minify(in), nil
}
