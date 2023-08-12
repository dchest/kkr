package importer

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/dchest/kkr/site"
	"gopkg.in/yaml.v3"
)

type pubDate struct {
	time.Time
}

func (d *pubDate) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	var s string
	if err := dec.DecodeElement(&s, &start); err != nil {
		return err
	}
	t, err := time.Parse("Mon, 02 Jan 2006 15:04:05 -0700", s)
	if err != nil {
		// In actual WP export I found date:
		// Tue, 30 Nov -0001 00:00:00 +0000
		// Let's just use zero time in this case.
		return nil
	}
	d.Time = t
	return nil
}

type wordpressPostMeta struct {
	Key   string `xml:"http://wordpress.org/export/1.2/ meta_key"`
	Value string `xml:"http://wordpress.org/export/1.2/ meta_value"`
}

type wordpressPost struct {
	Type        string              `xml:"http://wordpress.org/export/1.2/ post_type" yaml:"-"` // "post" or "page"
	Status      string              `xml:"http://wordpress.org/export/1.2/ status" yaml:"-"`    // "publish" or "draft"
	Slug        string              `xml:"http://wordpress.org/export/1.2/ post_name" yaml:"-"` // URL slug
	PubDate     pubDate             `xml:"pubDate" yaml:"date"`                                 // publication date
	Title       string              `xml:"title" yaml:"title"`
	Description string              `xml:"description" yaml:"description,omitempty"`
	Content     string              `xml:"http://purl.org/rss/1.0/modules/content/ encoded" yaml:"-"`
	Tags        []string            `xml:"category" yaml:"tags,omitempty,flow"`
	Meta        []wordpressPostMeta `xml:"http://wordpress.org/export/1.2/ postmeta" yaml:"-"`
}

func (p wordpressPost) IsPage() bool {
	return p.Type == "page"
}

func (p wordpressPost) IsPublished() bool {
	return p.Status == "publish"
}

func (p wordpressPost) Serialize() []byte {
	var buf bytes.Buffer
	buf.WriteString("---\n")
	// Serialize to YAML.
	if err := yaml.NewEncoder(&buf).Encode(p); err != nil {
		panic("yaml encode: " + err.Error()) // shouldn't happen
	}
	// Write "link_url" from wp:postmeta as "link".
	// This is a hack to convert my link blog.
	var linkWritten bool
	for _, m := range p.Meta {
		if !linkWritten && m.Key == "link_url" {
			linkMeta := map[string]string{"link": m.Value}
			if err := yaml.NewEncoder(&buf).Encode(linkMeta); err != nil {
				panic("yaml encode: " + err.Error()) // shouldn't happen
			}
			linkWritten = true
		}
	}
	buf.WriteString("---\n")
	// Add content.
	buf.WriteString(p.Content)
	return buf.Bytes()
}

func (p wordpressPost) Filename() string {
	if p.IsPage() {
		return p.Slug + ".html"
	}
	// Make filename in the format YYYY-MM-DD-slug.html
	return p.PubDate.Format("2006-01-02-") + p.Slug + ".html"
}

func (p wordpressPost) Dir() string {
	if !p.IsPublished() {
		return "drafts"
	}
	if p.IsPage() {
		return "pages"
	}
	return "posts"
}

func excludeStrings(source []string, excluded []string) []string {
	var result []string
	for _, s := range source {
		var found bool
		for _, e := range excluded {
			if s == e {
				found = true
				break
			}
		}
		if !found {
			result = append(result, s)
		}
	}
	return result
}

func readWordpressPosts(filename string) ([]wordpressPost, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var posts []wordpressPost
	dec := xml.NewDecoder(f)
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if tok.Name.Local == "item" {
				var post wordpressPost
				if err := dec.DecodeElement(&post, &tok); err != nil {
					return nil, err
				}
				posts = append(posts, post)
			}
		}
	}
	// Trim titles and content.
	for i := range posts {
		posts[i].Title = strings.TrimSpace(posts[i].Title)
		posts[i].Content = strings.TrimSpace(posts[i].Content)
		// Exclude some tag.
		// TODO: make an option for this.
		//posts[i].Tags = excludeStrings(posts[i].Tags, []string{"Uncategorized", "Link"})
	}
	return posts, nil
}

func makeDirs(base string, names ...string) error {
	for _, name := range names {
		if err := os.MkdirAll(path.Join(base, name), 0755); err != nil && err != os.ErrExist {
			return err
		}
	}
	return nil
}

func ImportWordpress(outDir string, filename string) error {
	posts, err := readWordpressPosts(filename)
	if err != nil {
		return err
	}
	// Make directories outDir and outDir/posts, outDir/drafts.
	if err := makeDirs(outDir, site.PostsDirName, site.PagesDirName, site.DraftsDirName); err != nil {
		return err
	}
	// Write posts.
	for _, p := range posts {
		b := p.Serialize()
		if err := os.WriteFile(path.Join(outDir, p.Dir(), p.Filename()), b, 0644); err != nil {
			return err
		}
	}
	return nil
}
