package hashcache

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func randomFilename() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err.Error())
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("%x", b[:]))
}

func TestSeen(t *testing.T) {
	path0 := "some/path"
	path1 := "another/path"
	content0 := []byte("some content to hash")
	content1 := []byte("some other content")

	c := New()
	res := c.Seen(path0, content0)
	if res {
		t.Errorf("0/0 update returned true, expected false")
	}
	res = c.Seen(path0, content0)
	if !res {
		t.Errorf("0/0 update returned false, expected true")
	}
	res = c.Seen(path1, content0)
	if res {
		t.Errorf("1/0 update returned true, expected false")
	}
	res = c.Seen(path1, content0)
	if !res {
		t.Errorf("1/0 update returned false, expected true")
	}
	res = c.Seen(path0, content1)
	if res {
		t.Errorf("0/1 update returned true, expected false")
	}

	// Write to file.
	filename := randomFilename()

	if err := c.WriteToFile(filename); err != nil {
		t.Errorf(err.Error())
	}
	defer os.Remove(filename)

	// Read and check.
	nc, err := NewFromFile(filename)
	if err != nil {
		t.Errorf(err.Error())
		return
	}
	res = nc.Seen(path1, content0)
	if !res {
		t.Errorf("1/0 update returned false, expected true")
	}
	res = nc.Seen(path0, content1)
	if !res {
		t.Errorf("0/1 update returned false, expected true")
	}
	res = nc.Seen("something", []byte("completely different"))
	if res {
		t.Errorf("update returned true, expected false")
	}
}

func BenchmarkSeen(b *testing.B) {
	c := New()
	b.ResetTimer()
	path := "path"
	content := make([]byte, 128)
	for i := 0; i < b.N; i++ {
		c.Seen(path, content)
	}
}

func BenchmarkSeen6(b *testing.B) {
	c := New()
	path0 := "some/kinda/long/path_to_file.txt"
	path1 := "other/path"
	content0 := make([]byte, 1024)
	content1 := make([]byte, 200)
	b.SetBytes(3 * int64(len(content0) + len(content1)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Seen(path0, content0)
		c.Seen(path1, content1)

		c.Seen(path0, content0)
		c.Seen(path1, content1)

		c.Seen(path0, content1)
		c.Seen(path1, content0)
	}
}
