package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"go.creack.net/gosh2/ast"
	"go.creack.net/gosh2/lexer"
	"go.creack.net/gosh2/parser"
)

type FileWrap struct {
	*os.File
}

func (f *FileWrap) Close() error {
	fmt.Printf("Checking if file %q is closed\n", f.Name())
	return f.File.Close()
}

func setupCommandIO(simpleCmd ast.SimpleCommand, cmd *exec.Cmd) error {
	for _, elem := range append(simpleCmd.Prefix.Redirects, simpleCmd.Suffix.Redirects...) {
		var openFlags int
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
		default:
			return fmt.Errorf("unsupported redirect %q", elem.Op)
		}

		var file *os.File
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
		} else {
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

func executePipeline(pipeline ast.Pipeline) (int, error) {
	var cmds []*exec.Cmd
	var simpleCmds []ast.SimpleCommand
	for _, pipeCmd := range pipeline.Commands {
		simpleCmd, _ := pipeCmd.(ast.SimpleCommand)
		simpleCmds = append(simpleCmds, simpleCmd)
		cmds = append(cmds, exec.Command(simpleCmd.Name, simpleCmd.Suffix.Words...))
	}

	lastCmd := cmds[len(cmds)-1]
	lastCmd.Stdout = os.Stdout
	lastCmd.Stderr = os.Stderr
	if err := setupCommandIO(simpleCmds[len(simpleCmds)-1], lastCmd); err != nil {
		return -1, fmt.Errorf("setup %q: %w", lastCmd.Path, err)
	}

	for i := len(cmds) - 1; i > 0; i-- {
		cmds[i].Stdin, _ = cmds[i-1].StdoutPipe()
		cmds[i-1].Stderr = os.Stderr
		if err := setupCommandIO(simpleCmds[i-1], cmds[i-1]); err != nil {
			return -1, fmt.Errorf("setup %q: %w", cmds[i-1].Path, err)
		}
	}

	for _, cmd := range cmds {
		if err := cmd.Start(); err != nil {
			return -1, fmt.Errorf("start %q: %w", cmd.Path, err)
		}
	}
	lastExitCode := -1
	const optPipefail = false
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

func evaluate(completeCmd ast.CompleteCommand) (int, error) {
	exitCode := -1
	_ = completeCmd.Separator
	for _, andOr := range completeCmd.List.AndOrs {
		_ = completeCmd.List.Separators
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
			lastExitCode, err := executePipeline(pipeline)
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

func test() (int, error) {
	input := "rm -f foo bar; ls -l > bar | foo.sh | wc -c | cat -e > foo; echo --; cat bar; echo --; cat foo"
	input = "echo hello > foo; echo world >> foo; ls /dev/fd 7<foo; cat /dev/fd/7 7<foo"
	input = "foo.sh 8> ret; echo why && echo ok1 || echo ko2 && echo ok2; cat ret; echo -1-"
	input = "echo hello > foo; foo.sh <> foo; echo --; cat foo"
	// input = "foo.sh 7<foo | cat -e; echo --; ls; echo --; cat a"
	// input = "cat /dev/fd/9 9<&7 7<foo"
	// input = "echo hello 8>bar >&8; cat bar"

	p := parser.New(strings.NewReader(input))
	lastExitCode := -1
	for {
		cmd := p.NextCompleteCommand()
		if cmd == nil {
			break
		}
		exitCode, err := evaluate(*cmd)
		if err != nil {
			log.Printf("evaluate error: %s.", err)
		}
		lastExitCode = exitCode
	}

	return lastExitCode, nil
}

func main() {
	for i := 3; i <= 7; i++ {
		_ = os.NewFile(uintptr(i), "").Close()
	}
	wd, _ := os.Getwd()
	os.Setenv("PATH", os.Getenv("PATH")+":"+wd)
	tmpDir, _ := os.MkdirTemp("/tmp", "gosh2")
	defer func() { _ = os.RemoveAll(tmpDir) }() // Best effort cleanup.
	if err := os.Chdir(tmpDir); err != nil {
		log.Fatalf("Fail: %s.", err)
	}
	if err := os.WriteFile("foo", []byte("foo\n"), 0644); err != nil {
		log.Fatalf("Fail: %s.", err)
	}

	lastExitCode, err := test()
	_ = os.RemoveAll(tmpDir) // Best effort cleanup.
	if err != nil {
		log.Fatalf("Fail: %s.", err)
	}
	os.Exit(lastExitCode)
}
