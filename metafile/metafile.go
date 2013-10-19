package metafile

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"sync"

	"github.com/dchest/goyaml"
)

const metaSeparator = "---\n"

type File struct {
	sync.Mutex
	f           *os.File
	r           *bufio.Reader
	metaRead    bool
	contentRead bool

	hasMeta bool
	meta    map[string]interface{}
	content []byte
}

func Open(name string) (m *File, err error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	m = &File{
		f: f,
		r: bufio.NewReader(f),
	}
	// Try reading meta.
	if err := m.readMeta(); err != nil {
		f.Close()
		return nil, err
	}
	return m, nil
}

func (m *File) Close() error {
	m.Lock()
	defer m.Unlock()
	return m.f.Close()
}

func (m *File) readMeta() error {
	m.Lock()
	defer m.Unlock()

	if m.metaRead {
		return nil
	}
	// Check if we have a meta file.
	p, err := m.r.Peek(len(metaSeparator))
	if (err != nil && err == io.EOF) || string(p) != metaSeparator {
		m.metaRead = true
		m.hasMeta = false
		return nil
	}
	if err != nil {
		return err
	}

	// Read meta.
	head, err := m.r.ReadString('\n')
	if err != nil {
		return err
	}
	if head != metaSeparator {
		// Shouldn't happen.
		panic("programmer error: head is not equal to meta separator")
	}
	buf := bytes.NewBuffer(nil)
	for {
		var s string
		s, err = m.r.ReadString('\n')
		if err != nil {
			return err
		}
		if s == metaSeparator {
			break
		}
		buf.WriteString(s)
	}
	m.meta = make(map[string]interface{})
	if err = goyaml.Unmarshal(buf.Bytes(), &m.meta); err != nil {
		return err
	}
	m.hasMeta = true
	m.metaRead = true
	return nil
}

func (m *File) Content() ([]byte, error) {
	m.Lock()
	defer m.Unlock()

	if m.contentRead {
		return m.content, nil
	}
	if !m.metaRead {
		panic("programmer error: meta wasn't read before reading content")
	}

	var buf bytes.Buffer
	_, err := io.Copy(&buf, m.r)
	if err != nil {
		return nil, err
	}
	m.content = buf.Bytes()
	m.contentRead = true
	return m.content, nil
}

func (m *File) HasMeta() bool {
	m.Lock()
	defer m.Unlock()
	if !m.metaRead {
		panic("programmer error: HasMeta called before ReadMeta")
	}
	return m.hasMeta
}

func (m *File) Meta() map[string]interface{} {
	m.Lock()
	defer m.Unlock()
	if !m.metaRead {
		panic("programmer error: Meta called before ReadMeta")
	}
	if !m.hasMeta {
		return nil
	}
	return m.meta
}
