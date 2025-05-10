package executor

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"go.creack.net/gosh2/ast"
	"go.creack.net/gosh2/lexer"
)

type FileWrap struct {
	*os.File
}

func (f *FileWrap) Close() error {
	fmt.Printf("Checking if file %q is closed\n", f.Name())
	return f.File.Close()
}

type CmdWrap struct {
	*exec.Cmd
}

func (c *CmdWrap) GetPath() string       { return c.Path }
func (c *CmdWrap) GetStdin() io.Reader   { return c.Stdin }
func (c *CmdWrap) GetStdout() io.Writer  { return c.Stdout }
func (c *CmdWrap) GetStderr() io.Writer  { return c.Stderr }
func (c *CmdWrap) SetStdin(r io.Reader)  { c.Stdin = r }
func (c *CmdWrap) SetStdout(w io.Writer) { c.Stdout = w }
func (c *CmdWrap) SetStderr(w io.Writer) { c.Stderr = w }

func (c *CmdWrap) GetExtraFD(n int) *os.File {
	if len(c.ExtraFiles) > n-3 {
		return c.ExtraFiles[n-3]
	}
	return nil
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

func setupCommandIO(aCmd ast.Command, cmd CmdIO) error {
	for _, elem := range aCmd.GetRedirects() {
		var openFlags int
		var file *os.File

		switch elem.Op {
		case lexer.TokRedirectLess:
			openFlags |= os.O_RDONLY
		case lexer.TokRedirectGreat, lexer.TokRedirectGreatAnd:
			openFlags |= os.O_CREATE | os.O_TRUNC | os.O_WRONLY
		case lexer.TokRedirectDoubleGreat:
			openFlags |= os.O_CREATE | os.O_APPEND | os.O_WRONLY
		case lexer.TokRedirectLessGreat:
			openFlags |= os.O_CREATE | os.O_RDWR
		case lexer.TokRedirectLessAnd:
		case lexer.TokRedirectDoubleLess:
			r, w, err := os.Pipe()
			if err != nil {
				return fmt.Errorf("heredoc pipe: %w", err)
			}
			go func() {
				defer func() { _ = w.Close() }() // Best effort.
				fmt.Fprint(w, elem.HereDoc)
			}()
			file = r
		default:
			return fmt.Errorf("unsupported redirect %q", elem.Op)
		}

		if elem.Filename != "" {
			// Check for invalid case `echo hello 4>& foo`.
			// The `>&` redirect only support '1' (or empty, which defaults to 1)
			// when used with a target filename.
			if elem.Op == lexer.TokRedirectGreatAnd && elem.Number != 1 {
				return fmt.Errorf("ambiguous redirect %q", elem.Op)
			}
			f, err := os.OpenFile(elem.Filename, openFlags, 0o644)
			if err != nil {
				return fmt.Errorf("openfile %q: %w", elem.Filename, err)
			}
			file = f
		} else if elem.ToNumber != nil {
			switch *elem.ToNumber {
			case 0:
				file, _ = cmd.GetStdin().(*os.File)
			case 1:
				file, _ = cmd.GetStdout().(*os.File)
			case 2:
				file, _ = cmd.GetStderr().(*os.File)
			default:
				file = cmd.GetExtraFD(*elem.ToNumber)
			}
			if file == nil {
				return fmt.Errorf("bad file descriptior %d\n", *elem.ToNumber)
			}
		} else if file == nil {
			return fmt.Errorf("missing filename or fd for %q", elem.Op)
		}

		switch elem.Number {
		case 0:
			cmd.SetStdin(file)
		case 1:
			cmd.SetStdout(file)
			// Case for `>& filename`, redirect both stdout and stderr to the file.
			if elem.Op == lexer.TokRedirectGreatAnd && elem.Filename != "" {
				cmd.SetStderr(file)
			}
		case 2:
			cmd.SetStderr(file)
		default:
			cmd.SetExtraFD(elem.Number, file)
		}
	}
	return nil
}

func executePipeline(pipeline ast.Pipeline, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	var cmds []CmdIO
	var simpleCmds []ast.Command
	// Create exec.Cmd for each command in the pipeline.
	for _, pipeCmd := range pipeline.Commands {
		switch c := pipeCmd.(type) {
		case ast.SimpleCommand:
			simpleCmds = append(simpleCmds, c)
			builtin := handleBuiltinCmd(c)
			if builtin != nil {
				cmds = append(cmds, builtin)
				continue
			}
			cmd := exec.Command(c.Name, c.Suffix.Words...)
			cmd.Env = append(os.Environ(), c.Prefix.Assignments...)
			cmds = append(cmds, &CmdWrap{cmd})
		case ast.CompoundCommand:
			switch c.Type {
			case "subshell":
				simpleCmds = append(simpleCmds, c)
				cmd := exec.Command(os.Args[0], "-sub")
				cmd.Env = os.Environ()
				cmd.Stdin = strings.NewReader(c.Body.Dump())
				cmds = append(cmds, &CmdWrap{cmd})
			default:
				return -1, fmt.Errorf("unsupported compound command type %q", c.Type)
			}
		default:
			return -1, fmt.Errorf("unsupported command type %T", c)
		}
	}

	// Set stdin/stdout/stderr for the last command.
	lastCmd := cmds[len(cmds)-1]
	if lastCmd.GetStdin() == nil {
		lastCmd.SetStdin(stdin)
	}
	lastCmd.SetStdout(stdout)
	lastCmd.SetStderr(stderr)

	// Handle io redirections for the last command.
	if err := setupCommandIO(simpleCmds[len(simpleCmds)-1], lastCmd); err != nil {
		return -1, fmt.Errorf("setup %q: %w", lastCmd.GetPath(), err)
	}

	// For every other command in the pipeline, hook stdin to the previous command's stdout.
	for i := len(cmds) - 1; i > 0; i-- {
		stdin, _ := cmds[i-1].StdoutPipe()
		cmds[i].SetStdin(stdin)
		cmds[i-1].SetStderr(stderr)
		if err := setupCommandIO(simpleCmds[i-1], cmds[i-1]); err != nil {
			return -1, fmt.Errorf("setup %q: %w", cmds[i-1].GetPath(), err)
		}
	}

	// Start all commands in the pipeline.
	for _, cmd := range cmds {
		if err := cmd.Start(); err != nil {
			return -1, fmt.Errorf("start %q: %w", cmd.GetPath(), err)
		}
	}
	// Wait on all commands in the pipeline. Keep track of the last exit code.
	lastExitCode := -1
	const optPipefail = false // TODO: Actually implement pipefail.
	for _, cmd := range cmds {
		err := cmd.Wait()
		if ps := cmd.GetProcessState(); ps != nil {
			lastExitCode = ps.ExitCode()
		}
		if err != nil && optPipefail {
			return lastExitCode, fmt.Errorf("wait %q: %w", cmd.GetPath(), err)
		}
	}

	return lastExitCode, nil
}

func evaluateAndOrs(andOrs []ast.AndOr, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	exitCode := -1
	for _, andOr := range andOrs {
		lastCmdSuccess := true
		for i, pipeline := range andOr.Pipelines {
			// If we have operators, check the last command's success/failure.
			if i > 0 && i-1 < len(andOr.Operators) {
				if andOr.Operators[i-1] == lexer.TokLogicalAnd && !lastCmdSuccess {
					continue
				}
				if andOr.Operators[i-1] == lexer.TokLogicalOr && lastCmdSuccess {
					continue
				}
			}
			lastExitCode, err := executePipeline(pipeline, stdin, stdout, stderr)
			if err != nil {
				log.Printf("Pipeline %v failed: %s.\n", pipeline, err)
			}
			lastCmdSuccess = err == nil && lastExitCode == 0
			if pipeline.Negated {
				lastCmdSuccess = !lastCmdSuccess
			}
			exitCode = lastExitCode
		}
	}

	return exitCode, nil
}

func Evaluate(completeCmd ast.CompleteCommand, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	_ = completeCmd.Separator // TODO: Handle separator (job control).
	return evaluateAndOrs(completeCmd.List.AndOrs, stdin, stdout, stderr)
}
