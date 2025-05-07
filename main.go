package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"

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
	inputFDs := map[int]struct{}{}
	outputFDs := map[int]struct{}{}
	for _, elem := range append(simpleCmd.Prefix.Redirects, simpleCmd.Suffix.Redirects...) {
		switch elem.Op {
		case lexer.TokRedirectIn:
			file, err := os.Open(elem.Filename)
			if err != nil {
				return fmt.Errorf("open %q: %w", elem.Filename, err)
			}
			switch elem.Number {
			case 0:
				cmd.Stdin = file
			default:
				if _, ok := outputFDs[elem.Number]; ok {
					return fmt.Errorf("cannot redirect %q to %d: already used for output", elem.Filename, elem.Number)
				}
				if len(cmd.ExtraFiles) <= elem.Number-3 {
					extraFiles := make([]*os.File, elem.Number-3+1)
					copy(extraFiles, cmd.ExtraFiles)
					cmd.ExtraFiles = extraFiles
				}
				cmd.ExtraFiles[elem.Number-3] = file
				inputFDs[elem.Number] = struct{}{}
			}
		case lexer.TokRedirectOut, lexer.TokDoubleRedirectOut:
			openFlags := os.O_WRONLY | os.O_CREATE
			if elem.Op == lexer.TokDoubleRedirectOut {
				openFlags |= os.O_APPEND
			} else {
				openFlags |= os.O_TRUNC
			}
			file, err := os.OpenFile(elem.Filename, openFlags, 0o644)
			if err != nil {
				return fmt.Errorf("openfile %q: %w", elem.Filename, err)
			}
			switch elem.Number {
			case 1:
				cmd.Stdout = file
			case 2:
				cmd.Stderr = file
			default:
				if _, ok := inputFDs[elem.Number]; ok {
					return fmt.Errorf("cannot redirect %d to %q: already used for input", elem.Number, elem.Filename)
				}
				if len(cmd.ExtraFiles) <= elem.Number-3 {
					extraFiles := make([]*os.File, elem.Number-3+1)
					copy(extraFiles, cmd.ExtraFiles)
					cmd.ExtraFiles = extraFiles
				}
				cmd.ExtraFiles[elem.Number-3] = file
				outputFDs[elem.Number] = struct{}{}
			}
		default:
			return fmt.Errorf("unsupported redirect %q", elem.Op)
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
		// TODO: Only fail if the last command fails.
		// TODO: Support `set -o pipefail` to fail if any command fails.
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

func evaluate(prog ast.Program) (int, error) {
	exitCode := -1
	for _, completeCmd := range prog.Commands {
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
	}
	return exitCode, nil
}

func test() error {
	input := "rm -f foo bar; ls -l > bar | foo.sh | wc -c | cat -e > foo; echo --; cat bar; echo --; cat foo"
	input = "echo hello > foo; echo world >> foo; ls /dev/fd 7<foo; cat /dev/fd/7 7<foo"
	input = "foo.sh 8> ret; echo why && echo ok1 || echo ko2 && echo ok2; cat ret; echo -1-"
	input = "sdafasdfdsaf; echo yup; echo ok;"
	prog := parser.Parse(lexer.New(input))
	exitCode, err := evaluate(prog)
	if err != nil {
		log.Printf("evaluate error: %s.", err)
	}
	os.Exit(exitCode)
	return err
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
	if err := os.WriteFile("foo", []byte("foo"), 0644); err != nil {
		log.Fatalf("Fail: %s.", err)
	}

	if err := test(); err != nil {
		log.Fatalf("Fail: %s.", err)
	}
}
