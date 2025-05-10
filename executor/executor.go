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
