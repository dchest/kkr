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
	dir           string
	excludeGlobs  []string
	state         map[string]os.FileInfo
	interval      time.Duration
	sleepInterval time.Duration
	closed        chan bool

	// event channels
	Change chan bool
	Error  chan error
}

const (
	DefaultInterval = 1 * time.Second
	SleepAfter      = 5 * time.Minute
)

// Watch polls the given directory and subdirectories and files inside it,
// excluding the given globs, for changes with the given interval.
//
// When there was no change for the given interval in 5 minutes, interval
// changes to sleepInterval (interval * 5 by default).
// It's back to normal interval if a change is detected.
// If sleepInterval is negative, don't sleep.
//
// It returns a Watcher or an error.
func Watch(dir string, excludeGlobs []string, interval, sleepInterval time.Duration) (w *Watcher, err error) {
	if interval == 0 {
		interval = DefaultInterval
	}
	if sleepInterval < 0 {
		sleepInterval = interval
	} else if sleepInterval == 0 {
		sleepInterval = DefaultInterval * 5
	}
	w = &Watcher{
		dir:           dir,
		excludeGlobs:  excludeGlobs,
		interval:      interval,
		sleepInterval: sleepInterval,
		Change:        make(chan bool),
		Error:         make(chan error),
		closed:        make(chan bool),
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
	lastChangeTime := time.Now()
	currentInterval := w.interval
	for {
		hasChange, err := w.check()
		switch {
		case err != nil:
			w.Error <- err
		case hasChange:
			now := time.Now()
			if now.Sub(lastChangeTime) > SleepAfter {
				currentInterval = w.sleepInterval
			} else {
				currentInterval = w.interval
			}
			lastChangeTime = now
			w.Change <- true
		}
		select {
		case <-time.After(currentInterval):
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
			if !matched {
				m, err := filepath.Match(glob, fi.Name())
				if err != nil {
					return err
				}
				matched = m
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
		// Compare modes.
		if ofi.Mode() != nfi.Mode() {
			return true, nil
		}
		if !ofi.IsDir() {
			// Compare times.
			if !ofi.ModTime().Equal(nfi.ModTime()) {
				return true, nil
			}
			// Compare sizes.
			if ofi.Size() != nfi.Size() {
				return true, nil
			}
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
