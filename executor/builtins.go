package executor

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"go.creack.net/gosh2/ast"
)

type builtinCmd struct {
	simpleCmd ast.SimpleCommand
	output    string

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	extraFiles []*os.File
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

func (c *builtinCmd) GetExtraFD(n int) *os.File {
	if len(c.extraFiles) > n-3 {
		return c.extraFiles[n-3]
	}
	return nil
}
func (c *builtinCmd) SetExtraFD(n int, file *os.File) {
	if len(c.extraFiles) <= n-3 {
		extraFiles := make([]*os.File, n-3+1)
		copy(extraFiles, c.extraFiles)
		c.extraFiles = extraFiles
	}
	c.extraFiles[n-3] = file
}

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

func newBuiltinEcho(scmd ast.SimpleCommand) *builtinCmd {
	cmd := newBuiltinCmd(scmd)
	cmd.output = strings.Join(cmd.simpleCmd.Suffix.Words, " ") + "\n"
	return cmd
}

func newBuiltinEnv(scmd ast.SimpleCommand) *builtinCmd {
	cmd := newBuiltinCmd(scmd)
	env := append(os.Environ(), scmd.Prefix.Assignments...)

	output := strings.Join(env, "\n") + "\n"
	cmd.output = output

	return cmd
}

type builtinExit struct {
	*builtinCmd
}

func (c *builtinExit) Start() error {
	if len(c.simpleCmd.Suffix.Words) == 0 {
		os.Exit(0)
		return nil
	}
	n, err := strconv.Atoi(c.simpleCmd.Suffix.Words[0])
	if err != nil {
		return fmt.Errorf("invalid exit code %q: %w", c.simpleCmd.Suffix.Words[0], err)
	}
	os.Exit(n)
	return nil
}

type builtinCD struct {
	*builtinCmd
}

func (c *builtinCD) Start() error {
	if len(c.simpleCmd.Suffix.Words) == 0 {
		// TODO: Handle this. Go back to last dir.
		return fmt.Errorf("cd: missing argument")
	}
	if err := os.Chdir(c.simpleCmd.Suffix.Words[0]); err != nil {
		log.Printf("cd: %s", err)
		return err
	}
	return nil
}

type builtinPWD struct {
	*builtinCmd
}

func (c *builtinPWD) Start() error {
	return nil
	if len(c.simpleCmd.Suffix.Words) != 0 {
		return fmt.Errorf("pwd: too many arguments")
	}
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("pwd: %w", err)
	}
	c.output = dir + "\n"
	return nil
}

func handleBuiltinCmd(scmd ast.SimpleCommand) CmdIO {
	switch scmd.Name {
	case "echo":
		return newBuiltinEcho(scmd)
	case "env":
		return newBuiltinEnv(scmd)
	case "exit":
		return &builtinExit{builtinCmd: newBuiltinCmd(scmd)}
	case "cd":
		return newBuiltinEcho(scmd)
		out := &builtinCD{builtinCmd: newBuiltinCmd(scmd)}
		out.output = "cd: " + strings.Join(scmd.Suffix.Words, " ") + "\n"
		return out
	case "pwd":
		return &builtinPWD{builtinCmd: newBuiltinCmd(scmd)}
	default:
		return nil
	}
}
