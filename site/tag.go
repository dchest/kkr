// Copyright 2022 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package site

import (
	"os"
	"path/filepath"

	"github.com/dchest/kkr/utils"
)

type TagIndex struct {
	Page
	Tag      string
	Filename string
	TagPosts Posts
}

func (p *TagIndex) Meta() map[string]interface{} { return p.meta }
func (p *TagIndex) Content() string              { return p.content }
func (p *TagIndex) FileInfo() os.FileInfo        { return nil }
func (p *TagIndex) URL() string                  { return p.url }

func NewTagIndex(tag, permalink string) *TagIndex {
	t := new(TagIndex)
	t.url = utils.CleanPermalink(permalink)
	t.content = tag
	t.meta = map[string]interface{}{"title": tag}
	t.Filename = filepath.FromSlash(utils.AddIndexIfNeeded(permalink))
	return t
}
