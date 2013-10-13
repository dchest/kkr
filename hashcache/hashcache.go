// Package hashcache tells whether the given content at the path was already seen by it.
package hashcache

import (
	"bufio"
	"crypto/md5"
	"encoding/binary"
	"fmt"
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

var hashFunc = md5.New

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
	w := bufio.NewWriter(f)
	// Write version.
	err = w.WriteByte(fileFormatVersion)
	if err != nil {
		return
	}
	c.Lock()
	defer c.Unlock()
	// Write length of map.
	err = binary.Write(w, binary.LittleEndian, uint64(len(c.m)))
	if err != nil {
		return
	}
	for path, hash := range c.m {
		// Write length of path.
		if len(path) > MaxPathLen {
			err = fmt.Errorf("path length %d is too big", len(path))
			return
		}
		err = binary.Write(w, binary.LittleEndian, uint16(len(path)))
		if err != nil {
			return
		}
		// Write path.
		_, err = w.WriteString(path)
		if err != nil {
			return
		}
		// Write hash.
		_, err = w.Write(hash[:])
		if err != nil {
			return
		}
	}
	err = w.Flush()
	return
}

func NewFromFile(filename string) (c *Cache, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()
	r := bufio.NewReader(f)
	// Read version.
	ver, err := r.ReadByte()
	if err != nil {
		return
	}
	if ver != fileFormatVersion {
		err = fmt.Errorf("wrong file format version: %d", ver)
		return
	}
	// Read length of map.
	var mapLen uint64
	err = binary.Read(r, binary.LittleEndian, &mapLen)
	if err != nil {
		return
	}
	// Limit this allocation hint to something reasonable.
	if mapLen > 100000 {
		mapLen = 100000
	}
	m := make(map[string][hashSize]byte, mapLen)
	var hash [hashSize]byte
	for {
		// Read length of path.
		var pathLen uint16
		err = binary.Read(r, binary.LittleEndian, &pathLen)
		if err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			return
		}
		if pathLen > MaxPathLen {
			return nil, fmt.Errorf("path length %d is too big", pathLen)
		}
		// Read path.
		pathBytes := make([]byte, int(pathLen))
		_, err = r.Read(pathBytes[:])
		if err != nil {
			return
		}
		// Read hash.
		_, err = r.Read(hash[:])
		if err != nil {
			return
		}
		// Set.
		m[string(pathBytes)] = hash
	}
	return &Cache{m: m, h: md5.New()}, nil
}
