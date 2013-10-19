package filters

// htmlmin is a primitive not-so-correct HTML minimizer.
func init() {
	RegisterMaker("htmlmin", func(_ []string) Filter { return HTMLMin(0) })
}

type HTMLMin int

func (f HTMLMin) Name() string { return "htmlmin" }

func (f HTMLMin) Filter(s string) (out string, err error) {
	var inTag, inQuote bool
	var prev byte
	b := make([]byte, 0, len(s))
	for _, c := range []byte(s) {
		ignoreThis := false
		switch c {
		case ' ', '\n', '\t':
			if !inQuote && (prev == ' ' || prev == '\n' || prev == '\t') {
				ignoreThis = true
			}
		case '<':
			inTag = true
		case '>':
			inTag = false
		case '"':
			if inQuote {
				inQuote = false
			} else if inTag {
				inQuote = true
			}
		}
		if !ignoreThis {
			b = append(b, c)
		}
		prev = c
	}
	return string(b), nil
}
