// Copyright 2014 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fspoll implements a primitive polling-based filesystem watcher.
package fspoll

import (
	"os"
	"path/filepath"
	"time"
)

type Watcher struct {
	dir          string
	excludeGlobs []string
	state        map[string]os.FileInfo
	interval     time.Duration
	closed       chan bool

	// event channels
	Change chan bool
	Error  chan error
}

const DefaultInterval = 1 * time.Second

// Watch polls the given directory and subdirectories and files inside it,
// exluding the given globs, for changes with the given interval.
//
// It returns a Watcher or an error.
func Watch(dir string, excludeGlobs []string, interval time.Duration) (w *Watcher, err error) {
	if interval == 0 {
		interval = DefaultInterval
	}
	w = &Watcher{
		dir:          dir,
		excludeGlobs: excludeGlobs,
		interval:     interval,
		Change:       make(chan bool),
		Error:        make(chan error),
		closed:       make(chan bool),
	}
	// Get initial state
	w.state, err = w.getState()
	if err != nil {
		return nil, err
	}
	// Start watching goroutine
	go w.start()
	return w, nil
}

func (w *Watcher) start() {
	for {
		hasChange, err := w.check()
		switch {
		case err != nil:
			w.Error <- err
		case hasChange:
			w.Change <- true
		}
		select {
		case <-time.After(w.interval):
			continue
		case <-w.closed:
			return
		}
	}
}

func (w *Watcher) getState() (map[string]os.FileInfo, error) {
	ns := make(map[string]os.FileInfo)
	err := filepath.Walk(w.dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		for _, glob := range w.excludeGlobs {
			matched, err := filepath.Match(glob, path)
			if err != nil {
				return err
			}
			if matched {
				// Skip excluded path
				if fi.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		ns[path] = fi
		return nil
	})
	return ns, err
}

func (w *Watcher) check() (hasChange bool, err error) {
	ns, err := w.getState()
	if err != nil {
		return false, err
	}
	defer func() {
		// Set new state as current when this function finishes.
		w.state = ns
	}()
	if len(ns) != len(w.state) {
		return true, nil
	}
	// Compare files.
	for path, nfi := range ns {
		ofi, ok := w.state[path]
		if !ok {
			// New file.
			return true, nil
		}
		// Compare times.
		if !ofi.ModTime().Equal(nfi.ModTime()) {
			return true, nil
		}
		// Compare sizes.
		if ofi.Size() != nfi.Size() {
			return true, nil
		}
		// Compare modes.
		if ofi.Mode() != nfi.Mode() {
			return true, nil
		}
	}
	// Check for deleted files.
	for opath := range w.state {
		_, ok := ns[opath]
		if !ok {
			return true, nil
		}
	}
	// Nothing changed.
	return false, nil
}

// Close stops the watcher.
func (w *Watcher) Close() {
	w.closed <- true
}
