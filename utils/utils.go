// Copyright 2013 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package utils contains utility functions.
package utils

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v1"
)

// UnmarshallYAMLFile reads YAML file and unmarshalls it into data.
func UnmarshallYAMLFile(filename string, data interface{}) error {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(b, data)
}

// CleanPermalink returns a cleaned version of permalink: without
// index.htm[l] ending and starting with slash.
func CleanPermalink(s string) string {
	// Strip index filename.
	if path.Base(s) == "index.html" || path.Base(s) == "index.htm" {
		s = s[:len(s)-len(path.Base(s))]
	}
	// Make sure it starts with /.
	if len(s) > 0 && s[0] != '/' {
		s = "/" + s
	}
	return s
}

// AddIndexIfNeeded adds "index.html" to the filename if
// it ends with a slash. Otherwise returns the original filename.
func AddIndexIfNeeded(filename string) string {
	if len(filename) == 0 || filename[len(filename)-1] != '/' {
		return filename
	}
	return filename + "index.html"
}

// StripEndSlash returns a string with ending slash removed,
// or if there was no slash, returns the original string.
func StripEndSlash(s string) string {
	// Remove ending slash.
	if len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

// Hash returns an SHA256 hash of the given string.
func Hash(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// TemplatedHash replaces ":hash" in template with hexadecimal characters of
// the hash of the input string and returns the result.
func TemplatedHash(template string, input []byte) string {
	// 10 bytes of hash is enough to avoid accidental collisions.
	hs := NoVowelsHexEncode(Hash(input)[:10])
	return strings.Replace(template, ":hash", hs, -1)
}

var dateTemplates = []string{
	"2006-01-02 15:04",
	"2006-01-02 15:04 -07:00",
	"2006-01-02 15:04:05 -07:00",
	time.RFC3339,
	time.RFC822,
	time.UnixDate,
	"2006.01.02 15:04",
	"2006.01.02",
	"2006-01-02",
}

// ParseAnyDate parses date in any of the few allowed formats.
func ParseAnyDate(s string) (d time.Time, err error) {
	for _, t := range dateTemplates {
		d, err = time.Parse(t, s)
		if err == nil {
			return
		}
	}
	err = fmt.Errorf("failed to parse date from %q", s)
	return
}

// DirExists returns true if the given directory exists.
func DirExist(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
}

// Returns true if filename has one of the given extension.
// Extensions must start with dot.
func HasFileExt(filename string, extensions []string) bool {
	ext := filepath.Ext(filename)
	for _, v := range extensions {
		if v == ext {
			return true
		}
	}
	return false
}

// ReplaceExtension replaces file extension with the given string.
// Extension must start with dot.
func ReplaceFileExt(filename string, ext string) string {
	oldext := filepath.Ext(filename)
	return filename[:len(filename)-len(oldext)] + ext
}

var absPathsRx = []*regexp.Regexp{
	regexp.MustCompile(`(?i)<([^>]+\s)(src|href)=(")/([^"]+)`),
	regexp.MustCompile(`(?i)<([^>]+\s)(src|href)=(')/([^']+)`),
	//TODO support non-quoted attribute values.
}

// AbsPaths adds urlPrefix to paths of src and href attributes
// in html starting with a slash (/).
func AbsPaths(urlPrefix, html string) string {
	urlPrefix = StripEndSlash(urlPrefix)
	repl := `<$1$2=${3}` + urlPrefix + `/$4`
	for _, re := range absPathsRx {
		html = re.ReplaceAllString(html, repl)
	}
	return html
}

// StripTags removes HTML tags.
// Extracted from https://github.com/kennygrant/sanitize
/*
License: BSD License
Copyright (c) 2013 Kenny Grant. All rights reserved.

Redistribution and use in source and binary forms, with or without modification,
are permitted provided that the following conditions are met:

   * Redistributions of source code must retain the above copyright notice,
this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above copyright notice,
this list of conditions and the following disclaimer in the documentation
and/or other materials provided with the distribution.
   * to endorse or promote products derived from this software
without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

func StripHTMLTags(s string) (output string) {
	// Shortcut strings with no tags in them
	if !strings.ContainsAny(s, "<>") {
		output = s
	} else {
		s = strings.Replace(s, "</p>", "\n", -1)
		s = strings.Replace(s, "<br>", "\n", -1)
		s = strings.Replace(s, "</br>", "\n", -1)

		b := bytes.NewBufferString("")
		inTag := false
		for _, r := range s {
			switch r {
			case '<':
				inTag = true
			case '>':
				inTag = false
			default:
				if !inTag {
					b.WriteRune(r)
				}
			}
		}
		output = b.String()
	}

	// In case we have missed any tags above, escape the text - removes <, >, &, ' and ".
	output = template.HTMLEscapeString(output)

	// Remove a few common harmless entities, to arrive at something more like plain text
	// This relies on having removed *all* tags above
	output = strings.Replace(output, "&nbsp;", " ", -1)
	output = strings.Replace(output, "&quot;", "\"", -1)
	output = strings.Replace(output, "&apos;", "'", -1)
	output = strings.Replace(output, "&#34;", "\"", -1)
	output = strings.Replace(output, "&#39;", "'", -1)
	// NB spaces here are significant - we only allow & not part of entity
	output = strings.Replace(output, "&amp; ", "& ", -1)
	output = strings.Replace(output, "&amp;amp; ", "& ", -1)

	return
}

// NoVowelsHexEncode returns bytes encoded in a hex-like encoding which
// doesn't use vowels.
//
// This is useful to avoid producing substrings, such as "ad", that
// may be blocked by ad-blockers.
func NoVowelsHexEncode(b []byte) string {
	const hextable = "0123456789vbcdzf"
	dst := make([]byte, len(b)*2)
	for i, v := range b {
		dst[i*2] = hextable[v>>4]
		dst[i*2+1] = hextable[v&0x0f]
	}
	return string(dst)
}

// OpenURL opens URL in the operating system (probably in the default browser).
func OpenURL(addr string) error {
	if _, err := url.Parse(addr); err != nil {
		return err
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", addr).Start()
	case "linux", "freebsd", "openbsd":
		return exec.Command("xdg-open", addr).Start()
	default:
		return fmt.Errorf("Don't know how to open browser on %s", runtime.GOOS)
	}
}

// Pool is a worker pool for parallel job processing.
type Pool struct {
	sync.Mutex
	wg   sync.WaitGroup
	jobs chan interface{}
	err  error
}

// NewPool creates a new pool which calls fn for each
// added item and stores the first returned error.
func NewPool(fn func(interface{}) error) *Pool {
	parallelism := runtime.NumCPU()
	p := &Pool{
		jobs: make(chan interface{}, parallelism),
	}
	// Launch workers.
	for i := 0; i < parallelism; i++ {
		go func() {
			for j := range p.jobs {
				err := fn(j)
				if err != nil {
					p.Lock()
					if p.err == nil {
						p.err = err
					}
					p.Unlock()
				}
				p.wg.Done()
			}
		}()
	}
	return p
}

// Add adds a new job to pool. Function passed to
// NewPool will be called for each job in a worker goroutine.
//
// After finishing adding items, Err must be called on the pool
// to wait for unfinished jobs to complete and get the first error.
func (p *Pool) Add(job interface{}) {
	p.wg.Add(1)
	p.jobs <- job
}

func (p *Pool) Err() error {
	p.wg.Wait()
	return p.err
}
