// Package hashcache tells whether the given content at the path was already seen by it.
package hashcache

import (
	"crypto/md5"
	"encoding/gob"
	"hash"
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
	m map[string][hashSize]byte
	h hash.Hash
}

func New() *Cache {
	return &Cache{m: make(map[string][hashSize]byte), h: md5.New()}
}

// contenHash returns hash of content. Cache must be locked.
func (c *Cache) contentHash(content []byte) (sum [hashSize]byte) {
	c.h.Reset()
	c.h.Write(content)
	c.h.Sum(sum[:0])
	return
}

// Seen sets content hash for the given path to a new value.
// It returns true if the content was already cached and had the same hash.
func (c *Cache) Seen(path string, content []byte) bool {
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

func (c *Cache) WriteToFile(filename string) (err error) {
	f, err := os.Create(filename)
	if err != nil {
		return
	}
	defer func() {
		f.Close()
		if err != nil {
			// Delete file.
			os.Remove(filename)
		}
	}()
	return gob.NewEncoder(f).Encode(c.m)
}

func NewFromFile(filename string) (c *Cache, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()
	m := make(map[string][hashSize]byte)
	err = gob.NewDecoder(f).Decode(&m)
	if err != nil {
		return
	}
	return &Cache{m: m, h: md5.New()}, nil
}
