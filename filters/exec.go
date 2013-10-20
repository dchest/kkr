package filters

// `exec` filter runs commands.

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
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

func (f *Exec) Apply(s string) (out string, err error) {
	cmd := exec.Command(f.command, f.args...)
	cmd.Stdin = strings.NewReader(s)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	err = cmd.Run()
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
