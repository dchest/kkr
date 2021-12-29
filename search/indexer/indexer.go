package indexer

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/dchest/stemmer/porter2"

	"github.com/dchest/kkr/search/indexer/tokenizer"
)

type Index struct {
	Docs       []*Document              `json:"docs"`
	Words      map[string][]interface{} `json:"words"`
	wordsToDoc map[string]map[int]float64

	HTMLTitleWeight        float64 `json:"-"`
	HTMLURLComponentWeight float64 `json:"-"`
}

type Document struct {
	URL   string `json:"u"`
	Title string `json:"t"`
}

func New() *Index {
	return &Index{
		Docs:                   make([]*Document, 0),
		Words:                  make(map[string][]interface{}),
		wordsToDoc:             make(map[string]map[int]float64), // words => doc => weight
		HTMLTitleWeight:        5,
		HTMLURLComponentWeight: 10,
	}
}

func (n *Index) WriteJSON(w io.Writer) error {
	for word, m := range n.wordsToDoc {
		for doc, weight := range m {
			// Normalize weight
			normWeight := int(weight * 1000)
			if normWeight < 1 {
				normWeight = 1
			}
			if normWeight == 1 {
				n.Words[word] = append(n.Words[word], doc)
			} else {
				n.Words[word] = append(n.Words[word], [2]int{doc, normWeight})
			}
		}
	}
	return json.NewEncoder(w).Encode(n)
}

func (n *Index) addWord(word string, doc int, weight float64) {
	m := n.wordsToDoc[word]
	if m == nil {
		m = make(map[int]float64)
		n.wordsToDoc[word] = m
	}
	m[doc] += weight
}

func (n *Index) newDocument(url, title string) int {
	n.Docs = append(n.Docs, &Document{URL: url, Title: title})
	return len(n.Docs) - 1
}

func stem(word string) string {
	if strings.ContainsAny(word, "0123456789") {
		return word // don't stem words with digits
	}
	return porter2.Stemmer.Stem(word)
}

func (n *Index) addString(doc int, text string, wordWeight float64) {
	wordcnt := make(map[string]float64)
	tk := tokenizer.Words(text)
	for tk.Next() {
		w := tk.Token()
		if len(w) < 1 || isStopWord(w) {
			continue
		}
		wordcnt[stem(removeAccents(w))] += wordWeight
		wordWeight /= 1.1
		if wordWeight < 0.0001 {
			wordWeight = 0.0001
		}
	}
	for w, c := range wordcnt {
		scaled := float64(c) / float64(len(wordcnt))
		if scaled < 0.0001 {
			scaled = 0.0001
		}
		n.addWord(w, doc, scaled) // scaled
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

func (n *Index) AddHTML(url string, r io.Reader) error {
	title, content, err := parseHTML(r)
	if err != nil {
		return err
	}
	doc := n.newDocument(url, title)
	n.addString(doc, title, n.HTMLTitleWeight)
	n.addString(doc, content, 1)
	// Add URL components.
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "www.")
	url = strings.ReplaceAll(url, "/", " ")
	n.addString(doc, url, n.HTMLURLComponentWeight)
	return nil
}
