// Copyright 2013 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package metafile implements reading of files with YAML headers.
package metafile

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v1"
)

const metaSeparator = "---"

type File struct {
	sync.Mutex
	fi          os.FileInfo
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
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	m = &File{
		fi: fi,
		f:  f,
		r:  bufio.NewReader(f),
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
	p, err := m.r.Peek(len(metaSeparator) + 1)
	if (err != nil && err == io.EOF) || strings.TrimSpace(string(p)) != metaSeparator {
		m.metaRead = true
		m.hasMeta = false
		return nil
	}
	if err != nil {
		return err
	}

	// Read meta.
	// Skip starting separator
	head, err := m.r.ReadString('\n')
	if err != nil {
		return err
	}
	if strings.TrimSpace(head) != metaSeparator {
		// Bad separator.
		return errors.New("Bad meta separator on the first line")
	}
	buf := bytes.NewBuffer(nil)
	for {
		var s string
		s, err = m.r.ReadString('\n')
		if err != nil {
			return err
		}
		if len(s) > 0 && strings.TrimSpace(s) == metaSeparator {
			break
		}
		buf.WriteString(s)
	}
	m.meta = make(map[string]interface{})
	if err = yaml.Unmarshal(buf.Bytes(), &m.meta); err != nil {
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

func (m *File) FileInfo() os.FileInfo {
	m.Lock()
	defer m.Unlock()
	return m.fi
}

// Changed returns true if file info on disk changed compared to the given file info.
func Changed(name string, fi os.FileInfo) bool {
	dfi, err := os.Stat(name)
	if err != nil {
		return true
	}
	// Check if file changed
	if fi.ModTime() != dfi.ModTime() {
		return true
	}
	if fi.Size() != dfi.Size() {
		return true
	}
	if fi.Mode() != dfi.Mode() {
		return true
	}
	return false
}
