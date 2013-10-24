// Copyright 2013 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metafile

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"
)

const (
	metaKey     = "title"
	metaValue   = "Hello world"
	metaContent = "This is\nsome content"

	fileWithMeta = metaSeparator + "\n" + metaKey + ": " + metaValue + "\n" + metaSeparator + "\n" + metaContent
	fileNoMeta   = `This isn't meta`
)

func WriteTempFile(s string) (name string, err error) {
	f, err := ioutil.TempFile("", "metafile-test-1-")
	if err != nil {
		return "", err
	}
	if _, err := io.WriteString(f, s); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	f.Close()
	return f.Name(), nil
}

func TestWithMeta(t *testing.T) {
	filename, err := WriteTempFile(fileWithMeta)
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer os.Remove(filename)

	m, err := Open(filename)
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer m.Close()
	if !m.HasMeta() {
		t.Errorf("HasMeta returned false, expecting true")
	}
	v, ok := m.Meta()[metaKey]
	if !ok || v.(string) != metaValue {
		t.Errorf("expecting %q: %q, got %q", metaKey, metaValue, v)
	}

	content, err := m.Content()
	if err != nil {
		t.Fatalf("%s", err)
	}

	if !bytes.Equal(content, []byte(metaContent)) {
		t.Errorf("content differs: expecting `%s`, got `%s`", metaContent, content)
	}
}

func TestNoMeta(t *testing.T) {
	filename, err := WriteTempFile(fileNoMeta)
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer os.Remove(filename)

	m, err := Open(filename)
	if err != nil {
		t.Fatalf("%s", err)
	}
	defer m.Close()
	if m.HasMeta() {
		t.Errorf("HasMeta returned true, expecting false")
	}
	content, err := m.Content()
	if err != nil {
		t.Fatalf("%s", err)
	}
	if !bytes.Equal(content, []byte(fileNoMeta)) {
		t.Errorf("content differs: expecting `%s`, got `%s`", metaContent, content)
	}
	// Try again.
	content, err = m.Content()
	if err != nil {
		t.Fatalf("%s", err)
	}
	if !bytes.Equal(content, []byte(fileNoMeta)) {
		t.Errorf("content differs: expecting `%s`, got `%s`", metaContent, content)
	}
}
