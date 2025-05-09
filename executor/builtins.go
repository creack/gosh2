package executor

import (
	"fmt"
	"io"
	"os"
	"strings"

	"go.creack.net/gosh2/ast"
)

type builtinCmd struct {
	simpleCmd ast.SimpleCommand
	output    string

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func newBuiltinCmd(cmd ast.SimpleCommand) *builtinCmd {
	return &builtinCmd{
		simpleCmd: cmd,
	}
}

func (c *builtinCmd) GetStdin() io.Reader  { return c.stdin }
func (c *builtinCmd) GetStdout() io.Writer { return c.stdout }
func (c *builtinCmd) GetStderr() io.Writer { return c.stderr }

func (c *builtinCmd) SetStdin(stdin io.Reader)   { c.stdin = stdin }
func (c *builtinCmd) SetStdout(stdout io.Writer) { c.stdout = stdout }
func (c *builtinCmd) SetStderr(stderr io.Writer) { c.stderr = stderr }

func (c *builtinCmd) GetExtraFD(n int) *os.File       { return nil }
func (c *builtinCmd) SetExtraFD(n int, file *os.File) {}

func (c *builtinCmd) GetProcessState() *os.ProcessState { return nil }

func (c *builtinCmd) GetPath() string { return c.simpleCmd.Name }

func (c *builtinCmd) StdoutPipe() (io.ReadCloser, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("pipe error: %w", err)
	}
	c.stdout = w
	return r, nil
}

func (c *builtinCmd) Start() error {
	if c.output != "" {
		if _, err := io.WriteString(c.stdout, c.output); err != nil {
			return err
		}
	}
	return nil
}

func (c *builtinCmd) Wait() error {
	if c.stdout != nil {
		closer, _ := c.stdout.(io.Closer)
		if closer != nil {
			return closer.Close()
		}
	}
	return nil
}

type builtinEcho struct {
	*builtinCmd
}

func newBuiltinEcho(scmd ast.SimpleCommand) *builtinEcho {
	cmd := &builtinEcho{
		builtinCmd: newBuiltinCmd(scmd),
	}
	cmd.output = strings.Join(cmd.simpleCmd.Suffix.Words, " ") + "\n"
	return cmd
}
