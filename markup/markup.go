package markup

import (
	"fmt"

	"stablelib.com/v1/blackfriday"
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
	return blackfriday.MarkdownCommon(content), nil
}
