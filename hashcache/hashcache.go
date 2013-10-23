// Copyright 2013 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//TODO: this package in currently unused, but may be used later.

// Package hashcache tells whether the given content at the path was already seen by it.
package hashcache

import (
	"crypto/md5"
	"encoding/gob"
	"hash"
	"io"
	"os"
	"sync"
)

const (
	hashSize          = md5.Size
	fileFormatVersion = 0

	MaxPathLen = 4096
)

type Cache struct {
	sync.Mutex
	filename string
	m        map[string][hashSize]byte
	h        hash.Hash
}

// contenHash returns hash of content. Cache must be locked.
func (c *Cache) contentHash(content string) (sum [hashSize]byte) {
	c.h.Reset()
	io.WriteString(c.h, content)
	c.h.Sum(sum[:0])
	return
}

// Clean cleans the cache.
func (c *Cache) Clean() {
	c.Lock()
	defer c.Unlock()
	c.m = make(map[string][hashSize]byte)
}

// Seen sets content hash for the given path to a new value.
// It returns true if the content was already cached and had the same hash.
func (c *Cache) Seen(path, content string) bool {
	c.Lock()
	defer c.Unlock()
	origHash, ok := c.m[path]
	newHash := c.contentHash(content)
	if !ok || origHash != newHash {
		c.m[path] = newHash
		return false
	}
	return true
}

func (c *Cache) Save() error {
	f, err := os.Create(c.filename)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := gob.NewEncoder(f).Encode(c.m); err != nil {
		// Delete file.
		os.Remove(c.filename)
		return err
	}
	return nil
}

func Open(filename string) (c *Cache, err error) {
	c = &Cache{
		filename: filename,
		m:        make(map[string][hashSize]byte),
		h:        md5.New(),
	}
	f, err := os.Open(filename)
	if err != nil && os.IsNotExist(err) {
		// No file, so create new empty cache.
		return c, nil
	}
	if err != nil {
		return
	}
	defer f.Close()
	err = gob.NewDecoder(f).Decode(&c.m)
	return
}
