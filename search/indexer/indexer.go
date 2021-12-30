package indexer

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/dchest/stemmer/porter2"

	"github.com/dchest/kkr/search/indexer/tokenizer"
)

type Index struct {
	Docs       []*Document              `json:"docs"`
	Words      map[string][]interface{} `json:"words"`
	wordsToDoc map[string]map[*Document]int

	ContentWordWeight      float64 `json:"-"`
	HTMLTitleWeight        float64 `json:"-"`
	HTMLURLComponentWeight float64 `json:"-"`
}

type Document struct {
	URL   string `json:"u"`
	Title string `json:"t"`

	numWords  int `json:"-"`
	selfIndex int `json:"-"`
}

func New() *Index {
	return &Index{
		Docs:                   make([]*Document, 0),
		Words:                  make(map[string][]interface{}),
		wordsToDoc:             make(map[string]map[*Document]int), // word => doc => weight
		ContentWordWeight:      1,
		HTMLTitleWeight:        3,
		HTMLURLComponentWeight: 3,
	}
}

func (n *Index) WriteJSON(w io.Writer) error {
	// Calculate minWeight (for normalization later),
	// and add word frequencies to doc.
	minWeight := math.MaxInt
	for _, m := range n.wordsToDoc {
		for doc, weight := range m {
			if weight < minWeight {
				minWeight = weight
			}
			doc.numWords += 1
		}
	}

	// Make JSON smaller:
	// Sort docs by most frequent, so that smaller
	// doc ids are taken for most frequent ones.
	sort.Slice(n.Docs, func(i, j int) bool {
		return n.Docs[i].numWords > n.Docs[j].numWords
	})

	// Add indexes to docs
	for i, d := range n.Docs {
		d.selfIndex = i
	}

	minWeight -= 1
	for word, m := range n.wordsToDoc {
		for doc, weight := range m {
			// Normalize weight.
			normWeight := weight - minWeight
			if normWeight == 1 {
				n.Words[word] = append(n.Words[word], doc.selfIndex)
			} else {
				n.Words[word] = append(n.Words[word], [2]interface{}{doc.selfIndex, normWeight})
			}
		}
	}
	return json.NewEncoder(w).Encode(n)
}

func (n *Index) addWord(word string, doc *Document, weight float64) {
	m := n.wordsToDoc[word]
	if m == nil {
		m = make(map[*Document]int)
		n.wordsToDoc[word] = m
	}
	m[doc] += int(weight * 100)
}

func (n *Index) newDocument(url, title string) *Document {
	doc := &Document{URL: url, Title: title}
	n.Docs = append(n.Docs, doc)
	return doc
}

func stem(word string) string {
	if strings.ContainsAny(word, "0123456789") {
		return word // don't stem words with digits
	}
	return porter2.Stemmer.Stem(word)
}

func (n *Index) addString(doc *Document, text string, wordWeight float64) {
	wordcnt := make(map[string]float64)
	tk := tokenizer.Words(text)
	for tk.Next() {
		w := normalizeWord(tk.Token())
		if len(w) < 1 || isStopWord(w) {
			continue
		}
		wordcnt[stem(w)] += wordWeight
	}
	// Scale each word weight and add it
	for w, c := range wordcnt {
		scaled := c / float64(len(wordcnt))
		n.addWord(w, doc, scaled)
	}
}

func (n *Index) AddText(url, title string, r io.Reader) error {
	var b bytes.Buffer
	if _, err := io.Copy(&b, r); err != nil {
		return err
	}
	n.addString(n.newDocument(url, title), b.String(), 1)
	return nil
}

func (n *Index) AddHTML(url string, r io.Reader) (indexed bool, err error) {
	title, content, indexable, err := parseHTML(r)
	if err != nil {
		return false, err
	}
	if !indexable {
		return false, nil
	}
	doc := n.newDocument(url, title)

	// Adjust word weight according to document level
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "www.")
	url = strings.TrimSuffix(url, "/")

	level := float64(strings.Count(url, "/"))
	if level == 0 {
		level = 1
	}

	n.addString(doc, title, n.HTMLTitleWeight/level)
	n.addString(doc, content, 0.5+0.5*(n.ContentWordWeight/level))
	// Add URL components.
	url = strings.ReplaceAll(url, "/", " ")
	url = strings.ReplaceAll(url, "_", " ")
	url = strings.ReplaceAll(url, "-", " ")
	n.addString(doc, url, n.HTMLURLComponentWeight/level)
	return true, nil
}
