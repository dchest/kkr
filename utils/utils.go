// Package utils contains utility functions.
package utils

import (
	"crypto/md5"
	"fmt"
	"github.com/dchest/goyaml"
	"path"
	"io/ioutil"
	"io"
	"os"
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
	if s[len(s)-1] == '/' {
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
	time.RFC3339,
	time.RFC822,
	time.UnixDate,
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

