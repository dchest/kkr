// Copyright 2013 Dmitry Chestnykh. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package filters

// `exec` filter runs commands.

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

func init() {
	Register("exec", func(args []string) Filter {
		return &Exec{command: args[0], args: args[1:]}
	})
}

type Exec struct {
	command string
	args    []string
}

func (f *Exec) Name() string { return fmt.Sprintf("exec %s %q", f.command, f.args) }

func (f *Exec) Apply(in []byte) (out []byte, err error) {
	cmd := exec.Command(f.command, f.args...)
	cmd.Stdin = bytes.NewReader(in)
	var buf bytes.Buffer
	var errbuf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &errbuf
	err = cmd.Run()
	if err != nil {
		errbuf.WriteTo(os.Stderr)
		return nil, fmt.Errorf("`%s` error: %s", f.Name(), err)
	}
	return buf.Bytes(), nil
}
