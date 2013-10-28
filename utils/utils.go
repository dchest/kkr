// Copyright 2013 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package utils contains utility functions.
package utils

import (
	"crypto/md5"
	"fmt"
	"github.com/dchest/goyaml"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"
)

// UnmarshallYAMLFile reads YAML file and unmarshalls it into data.
func UnmarshallYAMLFile(filename string, data interface{}) error {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return goyaml.Unmarshal(b, data)
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

// StripEndSlash returns a string with ending slash removed,
// or if there was no slash, returns the original string.
func StripEndSlash(s string) string {
	// Remove ending slash.
	if len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

// Hash returns an MD5 hash of the given string.
func Hash(s string) []byte {
	h := md5.New()
	io.WriteString(h, s)
	return h.Sum(nil)
}

// WriteStringToFile writes string to a file, making parent directories
// if they don't exist.
func WriteStringToFile(filename, data string) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return err
	}
	return ioutil.WriteFile(filename, []byte(data), 0644)
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
