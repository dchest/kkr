package main

import (
	"fmt"
	"path"
	"time"
)

func cleanPermalink(s string) string {
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

func cleanSiteURL(s string) string {
	// Remove ending slash.
	if s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

var dateTemplates = []string{
	"2006-01-02 15:04",
	time.RFC3339,
	time.RFC822,
	time.UnixDate,
	"2006-01-02",
}

func parseAnyDate(s string) (d time.Time, err error) {
	for _, t := range dateTemplates {
		d, err = time.Parse(t, s)
		if err == nil {
			return
		}
	}
	err = fmt.Errorf("failed to parse date from %q", s)
	return
}
