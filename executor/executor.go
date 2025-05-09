package executor

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
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

func setupCommandIO(aCmd ast.Command, cmd *exec.Cmd) error {
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
				file, _ = cmd.Stdin.(*os.File)
			case 1:
				file, _ = cmd.Stdout.(*os.File)
			case 2:
				file, _ = cmd.Stderr.(*os.File)
			default:
				if len(cmd.ExtraFiles) > *elem.ToNumber-3 {
					file = cmd.ExtraFiles[*elem.ToNumber-3]
				}
			}
			if file == nil {
				return fmt.Errorf("bad file descriptior %d\n", *elem.ToNumber)
			}
		} else if file == nil {
			return fmt.Errorf("missing filename or fd for %q", elem.Op)
		}

		switch elem.Number {
		case 0:
			cmd.Stdin = file
		case 1:
			cmd.Stdout = file
			// Case for `>& filename`, redirect both stdout and stderr to the file.
			if elem.Op == lexer.TokRedirectGreatAnd && elem.Filename != "" {
				cmd.Stderr = file
			}
		case 2:
			cmd.Stderr = file
		default:
			// Go pass down FDs via the ExtraFiles slice.
			// Any entry in the slice will start at fd 3 (i.e. after stdin, stdout, stderr).
			if len(cmd.ExtraFiles) <= elem.Number-3 {
				extraFiles := make([]*os.File, elem.Number-3+1)
				copy(extraFiles, cmd.ExtraFiles)
				cmd.ExtraFiles = extraFiles
			}
			cmd.ExtraFiles[elem.Number-3] = file
		}
	}
	return nil
}

func executePipeline(pipeline ast.Pipeline, stdout io.Writer) (int, error) {
	var cmds []*exec.Cmd
	var simpleCmds []ast.Command
	// Create exec.Cmd for each command in the pipeline.
	for _, pipeCmd := range pipeline.Commands {
		switch c := pipeCmd.(type) {
		case ast.SimpleCommand:
			simpleCmds = append(simpleCmds, c)
			cmd := exec.Command(c.Name, c.Suffix.Words...)
			if c.Name == "exit" {
				if len(c.Suffix.Words) == 0 {
					os.Exit(0)
					return -1, nil
				}
				n, err := strconv.Atoi(c.Suffix.Words[0])
				if err != nil {
					return -1, fmt.Errorf("invalid exit code %q: %w", c.Suffix.Words[0], err)
				}
				os.Exit(n)
				return -1, nil
			}
			cmd.Env = append(os.Environ(), c.Prefix.Assignments...)
			cmds = append(cmds, cmd)
		case ast.CompoundCommand:
			switch c.Type {
			case "subshell":
				simpleCmds = append(simpleCmds, c)
				cmd := exec.Command(os.Args[0], "-sub")
				cmd.Env = os.Environ()
				cmd.Stdin = strings.NewReader(c.Body.Dump())
				cmds = append(cmds, cmd)
			default:
			}
		default:
			return -1, fmt.Errorf("unsupported command type %T", c)
		}
	}

	// Set stdin/stdout/stderr for the last command.
	lastCmd := cmds[len(cmds)-1]
	if lastCmd.Stdin == nil {
		lastCmd.Stdin = os.Stdin
	}
	lastCmd.Stdout = stdout
	lastCmd.Stderr = os.Stderr
	// Handle io redirections for the last command.
	if err := setupCommandIO(simpleCmds[len(simpleCmds)-1], lastCmd); err != nil {
		return -1, fmt.Errorf("setup %q: %w", lastCmd.Path, err)
	}

	// For every other command in the pipeline, hook stdin to the previous command's stdout.
	for i := len(cmds) - 1; i > 0; i-- {
		cmds[i].Stdin, _ = cmds[i-1].StdoutPipe()
		cmds[i-1].Stderr = os.Stderr
		if err := setupCommandIO(simpleCmds[i-1], cmds[i-1]); err != nil {
			return -1, fmt.Errorf("setup %q: %w", cmds[i-1].Path, err)
		}
	}

	// Start all commands in the pipeline.
	for _, cmd := range cmds {
		if err := cmd.Start(); err != nil {
			return -1, fmt.Errorf("start %q: %w", cmd.Path, err)
		}
	}
	// Wait on all commands in the pipeline. Keep track of the last exit code.
	lastExitCode := -1
	const optPipefail = false // TODO: Actually implement pipefail.
	for _, cmd := range cmds {
		err := cmd.Wait()
		if cmd.ProcessState != nil {
			lastExitCode = cmd.ProcessState.ExitCode()
		}
		if err != nil && optPipefail {
			return lastExitCode, fmt.Errorf("wait %q: %w", cmd.Path, err)
		}
	}

	return lastExitCode, nil
}

func evaluateAndOrs(andOrs []ast.AndOr, stdout io.Writer) (int, error) {
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
			lastExitCode, err := executePipeline(pipeline, stdout)
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

func Evaluate(completeCmd ast.CompleteCommand, stdout io.Writer) (int, error) {
	_ = completeCmd.Separator
	return evaluateAndOrs(completeCmd.List.AndOrs, stdout)
}
