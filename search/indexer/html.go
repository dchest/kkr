package indexer

import (
	"bytes"
	"io"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type htmlParser struct {
	title   string
	b       bytes.Buffer
	noIndex bool // set to true if has meta name="robots" or name="kkr-search" and content="noindex">
}

func (p *htmlParser) parseMeta(n *html.Node) {
	consume := false
	robots := false
	for _, a := range n.Attr {
		if a.Key == "name" {
			v := strings.ToLower(a.Val)
			if v == "keywords" || v == "description" {
				consume = true
			} else if v == "robots" || v == "kkr-search" {
				robots = true
			}
		}
	}
	if !consume && !robots {
		return
	}
	for _, a := range n.Attr {
		if a.Key == "content" {
			if consume {
				p.consumeString(a.Val)
			} else if robots {
				p.noIndex = false
			}
		}
	}
}

func (p *htmlParser) skipNoIndexElement(n *html.Node) bool {
	for _, a := range n.Attr {
		if a.Key == "data-kkr-search" {
			v := strings.ToLower(a.Val)
			if v == "noindex" {
				return true
			}
		}
	}
	return false
}

func (p *htmlParser) parseImg(n *html.Node) {
	for _, a := range n.Attr {
		if a.Key == "alt" || a.Key == "title" {
			p.consumeString(a.Val)
			return
		}
	}
}

func (p *htmlParser) parseTitle(n *html.Node) {
	if c := n.FirstChild; c != nil && c.Type == html.TextNode {
		p.title = c.Data
	}
}

func (p *htmlParser) consumeString(s string) {
	p.b.WriteString(s)
	p.b.WriteByte('\n')
}

func (p *htmlParser) parseNoscript(n *html.Node) {
	c := n.FirstChild
	if c == nil {
		return
	}
	nodes, err := html.ParseFragment(strings.NewReader(c.Data), nil)
	if err != nil {
		return // ignore error
	}
	for _, v := range nodes {
		p.parseNode(v)
	}
}

func (p *htmlParser) parseNode(n *html.Node) {
	if p.noIndex {
		return
	}
	switch n.Type {
	case html.DocumentNode:
		// Parse children
		if c := n.FirstChild; c != nil {
			p.parseNode(c)
		}
	case html.ElementNode:
		switch n.DataAtom {
		case atom.Title:
			p.parseTitle(n)
		case atom.Meta:
			p.parseMeta(n)
		case atom.Img:
			p.parseImg(n)
		case atom.Noscript:
			// Parse insides of noscript as HTML.
			p.parseNoscript(n)
		case atom.Script, atom.Style:
			// skip children
		default:
			if !p.skipNoIndexElement(n) {
				// Parse children
				if c := n.FirstChild; c != nil {
					p.parseNode(c)
				}
			}
		}
	case html.TextNode:
		p.consumeString(n.Data)
	}
	// Parse sibling.
	if c := n.NextSibling; c != nil {
		p.parseNode(c)
	}
}

func (p *htmlParser) Parse(r io.Reader) error {
	doc, err := html.Parse(r)
	if err != nil {
		return err
	}
	p.parseNode(doc)
	return nil
}

func (p *htmlParser) Content() string {
	return p.b.String()
}

func (p *htmlParser) Title() string {
	return p.title
}

func (p *htmlParser) IsIndexable() bool {
	return !p.noIndex
}

func parseHTML(r io.Reader) (title, content string, indexable bool, err error) {
	var p htmlParser
	err = p.Parse(r)
	if err != nil {
		return
	}
	return p.Title(), p.Content(), p.IsIndexable(), nil
}
