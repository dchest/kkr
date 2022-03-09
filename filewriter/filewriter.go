package filewriter

import (
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/andybalholm/brotli"
)

// site.yaml -> compress:
type CompressConfig struct {
	Methods    []string `yaml:"methods"`
	Extensions []string `yaml:"extensions"`
}

type Compressor struct {
	Ext string
	New func(w io.Writer) io.WriteCloser
}

var gzipCompressor = &Compressor{
	Ext: "gz",
	New: func(w io.Writer) io.WriteCloser {
		z, err := gzip.NewWriterLevel(w, gzipLevel)
		if err != nil {
			panic(err.Error()) // shouldn't happen
		}
		return z
	},
}

var brotliCompressor = &Compressor{
	Ext: "br",
	New: func(w io.Writer) io.WriteCloser {
		return brotli.NewWriterLevel(w, brotliLevel)
	},
}

const (
	gzipLevel   = 9
	brotliLevel = 11
)

type FileWriter struct {
	compressedExtensions map[string]struct{}
	compressors          []*Compressor
}

func New(c *CompressConfig) (*FileWriter, error) {
	extensions := make(map[string]struct{})
	compressors := make([]*Compressor, 0)
	if c != nil {
		for _, v := range c.Extensions {
			extensions["."+v] = struct{}{}
		}
		for _, v := range c.Methods {
			switch v {
			case "gzip":
				compressors = append(compressors, gzipCompressor)
			case "br":
				compressors = append(compressors, brotliCompressor)
			default:
				return nil, fmt.Errorf("Unknown compression method: %q", v)
			}
		}
	}
	return &FileWriter{
		compressedExtensions: extensions,
		compressors:          compressors,
	}, nil
}

func (f *FileWriter) numberOfCompressors(ext string) int {
	if _, ok := f.compressedExtensions[ext]; ok {
		return len(f.compressors)
	}
	return 0
}

func (f *FileWriter) WriteFile(filename string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return err
	}
	nwriters := 1 + f.numberOfCompressors(filepath.Ext(filename))
	done := make(chan error, nwriters)
	go func() {
		done <- ioutil.WriteFile(filename, data, 0644)
	}()
	if nwriters > 1 {
		for _, c := range f.compressors {
			ext, newc := c.Ext, c.New
			go func() {
				out, err := os.OpenFile(filename+"."+ext, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
				if err != nil {
					done <- err
				}
				z := newc(out)
				if _, err := z.Write(data); err != nil {
					z.Close()
					out.Close()
					os.Remove(out.Name())
					done <- err
				}
				if err := z.Close(); err != nil {
					out.Close()
					os.Remove(out.Name())
					done <- err
				}
				if err := out.Close(); err != nil {
					os.Remove(out.Name())
					done <- err
				}
				done <- nil
			}()
		}
	}
	var lastErr error
	for i := 0; i < nwriters; i++ {
		err := <-done
		if err != nil && lastErr == nil {
			lastErr = err
		}
	}
	return lastErr
}

func compressFile(c *Compressor, filename string) (err error) {
	in, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer in.Close()
	outfile := filename + "." + c.Ext
	out, err := os.OpenFile(outfile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(outfile)
		}
	}()
	z := c.New(out)
	_, err = io.Copy(z, in)
	if err != nil {
		return err
	}
	err = z.Close()
	if err != nil {
		return err
	}
	return nil
}

func copyFile(outfile, infile string) (err error) {
	// Remove old outfile, ignoring errors.
	os.Remove(outfile)

	// Try making hard link instead of copying.
	if err := os.Link(infile, outfile); err == nil {
		return nil // success
	}

	// Failed to create hard link, so try copying content.
	in, err := os.Open(infile)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(outfile)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(outfile)
		}
	}()
	_, err = io.Copy(out, in)
	return err
}

func (f *FileWriter) CopyFile(outfile, infile string) error {
	if err := os.MkdirAll(filepath.Dir(outfile), 0755); err != nil {
		return err
	}

	// Copy.
	if err := copyFile(outfile, infile); err != nil {
		return err
	}

	// Compress.
	n := f.numberOfCompressors(filepath.Ext(outfile))
	if n == 0 {
		return nil
	}
	done := make(chan error, n)
	for _, c := range f.compressors {
		c := c
		go func() {
			done <- compressFile(c, outfile)
		}()
	}
	var lastErr error
	for i := 0; i < n; i++ {
		err := <-done
		if err != nil && lastErr == nil {
			lastErr = err
		}
	}
	return lastErr
}
