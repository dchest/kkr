package main

import (
	"path"
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
