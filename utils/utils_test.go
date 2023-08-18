package utils

import "testing"

func TestAbsPaths(t *testing.T) {
	var tests = []struct{ in, out string }{
		{
			`Hello <img class="img-responsive" src="/img/hello.png" alt=""> world!`,
			`Hello <img class="img-responsive" src="http://example.com/img/hello.png" alt=""> world!`,
		},
		{
			`Click <a href="/go/to">this\nlink</a>! But not <a href="http://google.com/calendar">this</a>.`,
			`Click <a href="http://example.com/go/to">this\nlink</a>! But not <a href="http://google.com/calendar">this</a>.`,
		},
	}
	for i, v := range tests {
		out := AbsPaths("http://example.com", v.in)
		if v.out != out {
			t.Errorf("%d: expected\n%s\ngot\n%s\n", i, v.out, out)
		}
	}
}

func TestToSlug(t *testing.T) {
	var tests = []struct{ in, out string }{
		{"Hello, world!", "hello-world"},
		{"Hello, свете!", "hello"},
	}
	for i, v := range tests {
		out := ToSlug(v.in)
		if v.out != out {
			t.Errorf("%d: expected %q, got %q", i, v.out, out)
		}
	}
}
