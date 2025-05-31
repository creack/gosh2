package executor

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

type CmdWrap struct {
	*exec.Cmd
}

func (c *CmdWrap) GetPath() string      { return c.Path }
func (c *CmdWrap) GetStdin() io.Reader  { return c.Stdin }
func (c *CmdWrap) GetStdout() io.Writer { return c.Stdout }
func (c *CmdWrap) GetStderr() io.Writer { return c.Stderr }
func (c *CmdWrap) SetStdin(r io.Reader) { c.Stdin = r }
func (c *CmdWrap) SetStdout(w io.Writer) {
	// if w != nil {
	// 	if _, ok := w.(*os.File); ok {
	// 		//	fmt.Printf("SET STDOUT %q TO %d\n", c.Cmd, w.(*os.File).Fd())
	// 	}
	// }
	c.Stdout = w
}
func (c *CmdWrap) SetStderr(w io.Writer) { c.Stderr = w }

func (c *CmdWrap) GetExtraFD(n int) *os.File {
	if len(c.ExtraFiles) > n-3 {
		return c.ExtraFiles[n-3]
	}
	return os.NewFile(uintptr(n), fmt.Sprintf("fd:%d", n))
}

func (c *CmdWrap) SetExtraFD(n int, file *os.File) {
	// Go pass down FDs via the ExtraFiles slice.
	// Any entry in the slice will start at fd 3 (i.e. after stdin, stdout, stderr).\
	if len(c.ExtraFiles) <= n-3 {
		extraFiles := make([]*os.File, n-3+1)
		copy(extraFiles, c.ExtraFiles)
		c.ExtraFiles = extraFiles
	}
	c.ExtraFiles[n-3] = file
}

func (c *CmdWrap) GetProcessState() Exiter { return c.ProcessState }

type Exiter interface {
	ExitCode() int
}

type CmdIO interface {
	GetStdin() io.Reader
	GetStdout() io.Writer
	GetStderr() io.Writer

	SetStdin(io.Reader)
	SetStdout(io.Writer)
	SetStderr(io.Writer)

	GetExtraFD(n int) *os.File
	SetExtraFD(n int, file *os.File)

	GetProcessState() Exiter

	GetPath() string

	StdoutPipe() (io.ReadCloser, error)
	Start() error
	Wait() error
}
