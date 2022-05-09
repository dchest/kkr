package markup

import (
	"fmt"

	"github.com/russross/blackfriday/v2"
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
	htmlFlags := blackfriday.CommonHTMLFlags

	if options.MarkdownAngledQuotes {
		htmlFlags |= blackfriday.SmartypantsAngledQuotes
	}

	extensions := blackfriday.CommonExtensions | blackfriday.LaxHTMLBlocks

	renderer := blackfriday.NewHTMLRenderer(blackfriday.HTMLRendererParameters{Flags: htmlFlags})
	return blackfriday.Run(content, blackfriday.WithExtensions(extensions), blackfriday.WithRenderer(renderer)), nil
}
