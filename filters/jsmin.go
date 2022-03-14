// Copyright 2013 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package filters

// `jsmin` minifies JavaScript.

import (
	"github.com/dchest/jsmin"
)

func init() {
	Register("jsmin", func(args []string) Filter {
		return JSMin(0)
	})
}

type JSMin int

func (f JSMin) Name() string { return "jsmin" }

func (f JSMin) Apply(in []byte) (out []byte, err error) {
	return jsmin.Minify(in)
}
