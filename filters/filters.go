// Copyright 2013 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package filters implements text filtering.
package filters

import (
	"fmt"
)

// Filter is an interface declaring a filter.
type Filter interface {
	Name() string
	Apply([]byte) ([]byte, error)
}

// Maker is a type of function which accepts arguments
// for filter and returns a new instance of the filter.
type Maker func([]string) Filter

// makers stores builtin filter makers addressed by their names.
var makers = make(map[string]Maker)

// Register registers a new filter maker.
func Register(name string, maker Maker) {
	makers[name] = maker
}

// Make creates a new filter by name with the given arguments.
// It returns nil if it can't find a filter maker with such name.
func Make(name string, args []string) Filter {
	maker := makers[name]
	if maker == nil {
		return nil
	}
	return maker(args)
}

// Collection is a collection of filters addressed by some key.
type Collection struct {
	filters map[string]Filter
	enabled bool
}

// NewCollection returns a new collection.
func NewCollection() *Collection {
	return &Collection{
		filters: make(map[string]Filter),
		enabled: true,
	}
}

// SetEnabled sets enabled state of the collection.
func (c *Collection) SetEnabled(enabled bool) {
	c.enabled = enabled
}

// Add adds the filter to collection to be addressable by key.
func (c *Collection) Add(key string, filterName string, args []string) error {
	f := Make(filterName, args)
	if f == nil {
		return fmt.Errorf("filter %s not found", filterName)
	}
	c.filters[key] = f
	return nil
}

// AddFromYAML parses a `filters` value (line) and adds corresponding filters.
func (c *Collection) AddFromYAML(key string, line interface{}) error {
	switch x := line.(type) {
	case string:
		return c.Add(key, x, nil)
	case []interface{}:
		args := make([]string, len(x))
		for i, v := range x {
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("failed to parse filters: not an array of strings")
			}
			args[i] = s
		}
		return c.Add(key, args[0], args[1:])
	default:
		return fmt.Errorf("failed to parse filters: not a string or array")
	}
}

// Get returns a filter for key.
// It returns nil if the filter wasn't found.
func (c *Collection) Get(key string) Filter {
	return c.filters[key]
}

// ApplyFilter applies a filter found by key to the given string.
// If the filter wasn't found, returns the original string.
func (c *Collection) ApplyFilter(key string, in []byte) (out []byte, err error) {
	f := c.filters[key]
	if f == nil {
		return in, nil
	}
	return f.Apply(in)
}
