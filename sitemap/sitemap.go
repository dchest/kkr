package sitemap

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"text/template"
)

type Entry struct {
	Loc        string
	Lastmod    string
	Changefreq string
	Priority   string
}

type Sitemap struct {
	entries []Entry
}

func New() *Sitemap {
	return &Sitemap{
		entries: make([]Entry, 0),
	}
}

func (m *Sitemap) Add(entry Entry) error {
	if !isValidChangefreq(entry.Changefreq) {
		return fmt.Errorf("invalid changefreq '%s'", entry.Changefreq)
	}
	m.entries = append(m.entries, entry)
	return nil
}

func (m *Sitemap) Render(w io.Writer, baseURL string) error {
	sort.Slice(m.entries, func(i, j int) bool {
		return len(m.entries[i].Loc) < len(m.entries[j].Loc)
	})

	return sitemapTemplate.Execute(w, struct {
		BaseURL string
		Entries []Entry
	}{
		baseURL,
		m.entries,
	})
}

func (m *Sitemap) Reset() {
	m.entries = m.entries[:0]
}

func isValidChangefreq(changefreq string) bool {
	for _, v := range validChangefreqs {
		if v == changefreq {
			return true
		}
	}
	return false
}

var validChangefreqs = [...]string{"", "always", "hourly", "daily", "weekly", "monthly", "yearly", "never"}

var sitemapFuncs = template.FuncMap{
	// `xml` function escapes XML.
	"xml": func(in string) (string, error) {
		var buf bytes.Buffer
		if err := xml.EscapeText(&buf, []byte(in)); err != nil {
			return "", err
		}
		return buf.String(), nil
	},
}

var sitemapTemplate = template.Must(template.New("").Funcs(sitemapFuncs).Parse(
	`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
{{- range .Entries}}
 <url>
  <loc>{{$.BaseURL | xml}}{{.Loc | xml}}</loc>
  {{- with .Lastmod}}
  <lastmod>{{. | xml}}</lastmod>
  {{- end}}
  {{- with .Changefreq}}
  <changefreq>{{. | xml}}</changefreq>
  {{- end}}
  {{- with .Priority}}
  <priority>{{. | xml}}</priority>
  {{- end}}
 </url>
</urlset>
{{- end}}
`))
