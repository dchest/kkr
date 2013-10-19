package filters

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func init() {
	RegisterMaker("exec", MakeExecFilter)
}

type execFilter struct {
	command string
	args    []string
}

func MakeExecFilter(args []string) Filter {
	return &execFilter{command: args[0], args: args[1:]}
}

func (f *execFilter) Name() string { return fmt.Sprintf("exec %s %q", f.command, f.args) }

func (f *execFilter) Filter(s string) (out string, err error) {
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
