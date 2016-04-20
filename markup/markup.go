package markup

import (
	"fmt"

	"github.com/russross/blackfriday"
)

type Options struct {
	MarkdownAngledQuotes bool `yaml:"markdown_angled_quotes"`
}

var options *Options

func SetOptions(opts *Options) {
	options = opts
}

func Process(markupName string, content []byte) ([]byte, error) {
	switch markupName {
	case "markdown":
		return processMarkdown(content)
	default:
		return nil, fmt.Errorf("unknown markup: %q", markupName)
	}
}

func processMarkdown(content []byte) ([]byte, error) {
	// Copy of commonHtmlFlags
	htmlFlags := 0 |
		blackfriday.HTML_USE_XHTML |
		blackfriday.HTML_USE_SMARTYPANTS |
		blackfriday.HTML_SMARTYPANTS_FRACTIONS |
		blackfriday.HTML_SMARTYPANTS_DASHES |
		blackfriday.HTML_SMARTYPANTS_LATEX_DASHES

	if options.MarkdownAngledQuotes {
		htmlFlags |= blackfriday.HTML_SMARTYPANTS_ANGLED_QUOTES
	}

	// Copy of commonExtension
	extensions := 0 |
		blackfriday.EXTENSION_NO_INTRA_EMPHASIS |
		blackfriday.EXTENSION_TABLES |
		blackfriday.EXTENSION_FENCED_CODE |
		blackfriday.EXTENSION_AUTOLINK |
		blackfriday.EXTENSION_STRIKETHROUGH |
		blackfriday.EXTENSION_SPACE_HEADERS |
		blackfriday.EXTENSION_HEADER_IDS |
		blackfriday.EXTENSION_BACKSLASH_LINE_BREAK |
		blackfriday.EXTENSION_DEFINITION_LISTS

	renderer := blackfriday.HtmlRenderer(htmlFlags, "", "")
	return blackfriday.MarkdownOptions(content, renderer, blackfriday.Options{Extensions: extensions}), nil
}
