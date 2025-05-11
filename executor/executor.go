package executor

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"go.creack.net/gosh2/ast"
	"go.creack.net/gosh2/lexer"
)

// func executePipeline(pipeline ast.Pipeline, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
// 	var cmds []CmdIO
// 	var simpleCmds []ast.Command
// 	// Create exec.Cmd for each command in the pipeline.
// 	for _, pipeCmd := range pipeline.Commands {
// 		switch c := pipeCmd.(type) {
// 		case ast.SimpleCommand:
// 			simpleCmds = append(simpleCmds, c)
// 			builtin := handleBuiltinCmd(c)
// 			if builtin != nil {
// 				cmds = append(cmds, builtin)
// 				continue
// 			}
// 			cmd := exec.Command(c.Name, c.Suffix.Words...)
// 			cmd.Env = append(os.Environ(), c.Prefix.Assignments...)
// 			cmds = append(cmds, &CmdWrap{cmd})
// 		case ast.CompoundCommand:
// 			switch c.Type {
// 			case "subshell":
// 				simpleCmds = append(simpleCmds, c)
// 				cmd := exec.Command(os.Args[0], "-sub")
// 				cmd.Env = os.Environ()
// 				cmd.Stdin = strings.NewReader(c.Body.Dump())
// 				cmds = append(cmds, &CmdWrap{cmd})
// 			default:
// 				return -1, fmt.Errorf("unsupported compound command type %q", c.Type)
// 			}
// 		default:
// 			return -1, fmt.Errorf("unsupported command type %T", c)
// 		}
// 	}

// 	// Set stdin/stdout/stderr for the last command.
// 	lastCmd := cmds[len(cmds)-1]
// 	if lastCmd.GetStdin() == nil {
// 		lastCmd.SetStdin(stdin)
// 	}
// 	lastCmd.SetStdout(stdout)
// 	lastCmd.SetStderr(stderr)

// 	// Handle io redirections for the last command.
// 	if err := setupCommandIO(simpleCmds[len(simpleCmds)-1], lastCmd); err != nil {
// 		return -1, fmt.Errorf("setup %q: %w", lastCmd.GetPath(), err)
// 	}

// 	// For every other command in the pipeline, hook stdin to the previous command's stdout.
// 	for i := len(cmds) - 1; i > 0; i-- {
// 		stdin, _ := cmds[i-1].StdoutPipe()
// 		cmds[i].SetStdin(stdin)
// 		cmds[i-1].SetStderr(stderr)
// 		if err := setupCommandIO(simpleCmds[i-1], cmds[i-1]); err != nil {
// 			return -1, fmt.Errorf("setup %q: %w", cmds[i-1].GetPath(), err)
// 		}
// 	}

// 	// Start all commands in the pipeline.
// 	for _, cmd := range cmds {
// 		if err := cmd.Start(); err != nil {
// 			return -1, fmt.Errorf("start %q: %w", cmd.GetPath(), err)
// 		}
// 	}
// 	// Wait on all commands in the pipeline. Keep track of the last exit code.
// 	lastExitCode := -1
// 	const optPipefail = false // TODO: Actually implement pipefail.
// 	for _, cmd := range cmds {
// 		err := cmd.Wait()
// 		if ps := cmd.GetProcessState(); ps != nil {
// 			lastExitCode = ps.ExitCode()
// 		}
// 		if err != nil && optPipefail {
// 			return lastExitCode, fmt.Errorf("wait %q: %w", cmd.GetPath(), err)
// 		}
// 	}

// 	return lastExitCode, nil
// }

func evaluateSimpleCommand(scmd *ast.SimpleCommand, stdin io.Reader, stdout, stderr io.Writer) (CmdIO, error) {
	var args []string
	if scmd.Suffix != nil {
		args = scmd.Suffix.Words()
	}
	cmd := exec.Command(scmd.Name, args...)
	cmd.Stdin = stdin
	// NOTE: Stdout setup later.
	cmd.Stderr = stderr
	cmd.Env = os.Environ()
	if scmd.Prefix != nil {
		cmd.Env = append(cmd.Env, scmd.Prefix.AssignmentWords()...)
	}

	return &CmdWrap{cmd}, nil
}

func evaluateCommand(cmd ast.Command, stdin io.Reader, stdout, stderr io.Writer) (CmdIO, error) {
	switch c := cmd.(type) {
	case *ast.SimpleCommand:
		return evaluateSimpleCommand(c, stdin, stdout, stderr)
	default:
		panic(fmt.Errorf("unsupported command type %T", c))
	}
}

func evaluatePipelineSequence(seq *ast.PipelineSequence, cmds *[]CmdIO, stdin io.Reader, stdout, stderr io.Writer) CmdIO {
	// If there is no left side, we only have a command.
	exCmd, err := evaluateCommand(seq.Right, stdin, stdout, stderr)
	if err != nil {
		// TODO: Handle this error.
		panic(err)
	}
	*cmds = append(*cmds, exCmd)
	if seq.Left != nil {
		pipeout, err := exCmd.StdoutPipe()
		if err != nil {
			// TODO: Handle this error.
			panic(err)
		}
		if err := setupCommandIO(seq.Right, exCmd); err != nil {
			// TODO: Handle this error.
			panic(err)
		}
		return evaluatePipelineSequence(seq.Left, cmds, pipeout, stdout, stderr)
	}

	exCmd.SetStdout(stdout)
	if err := setupCommandIO(seq.Right, exCmd); err != nil {
		// TODO: Handle this error.
		panic(err)
	}
	return exCmd
}

func evaluatePipeline(pipeline *ast.Pipeline, stdin io.Reader, stdout, stderr io.Writer) (int, bool, error) {
	cmds := []CmdIO{}
	evaluatePipelineSequence(pipeline.Right, &cmds, stdin, stdout, stderr)
	// Start all commands in the pipeline.
	for _, cmd := range cmds {
		if err := cmd.Start(); err != nil {
			return -1, false, fmt.Errorf("start %q: %w", cmd.GetPath(), err)
		}
	}
	// Wait on all commands in the pipeline. Keep track of the last exit code.
	lastExitCode := -1
	var lastErr error
	const optPipefail = false // TODO: Actually implement pipefail.
	for _, cmd := range cmds {
		err := cmd.Wait()
		if ps := cmd.GetProcessState(); ps != nil {
			lastExitCode = ps.ExitCode()
		}
		if err != nil && optPipefail {
			return lastExitCode, pipeline.Negated, fmt.Errorf("wait %q: %w", cmd.GetPath(), err)
		}
		lastErr = err
	}

	success := lastExitCode == 0 && lastErr == nil
	if pipeline.Negated {
		success = !success
	}
	return lastExitCode, success, nil
}

func evaluateAndOr(andOr *ast.AndOr, stdin io.Reader, stdout, stderr io.Writer) (int, bool, error) {
	// If there is no left side, we only have a pipeline.
	if andOr.Left == nil {
		return evaluatePipeline(andOr.Right, stdin, stdout, stderr)
	}
	if andOr.Separator == 0 { // Should never happen.
		panic("missing andor separator")
	}
	// Otherwise, recurse into the left side.
	exitCode, success, err := evaluateAndOr(andOr.Left, stdin, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "eval andor: %s\n", err)
	}
	// If we are in a AND and have a failure, stop here.
	if andOr.Separator == lexer.TokAndIf && !success {
		return exitCode, success, nil
	}
	// If we are in a OR and have a success, stop here.
	if andOr.Separator == lexer.TokOrIf && success {
		return exitCode, success, nil
	}
	// Otherwise, execute the right side.
	return evaluatePipeline(andOr.Right, stdin, stdout, stderr)
}

func evaluateList(list *ast.List, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if list.Left != nil {
		exitCode, err := evaluateList(list.Left, stdin, stdout, stderr)
		if err != nil {
			return exitCode, err
		}
	}
	// TODO: Handle separator (job control).
	if list.Separator == lexer.TokAmpersand {
		panic("job control not implemented")
	}
	if list.Right != nil {
		exitCode, _, err := evaluateAndOr(list.Right, stdin, stdout, stderr)
		return exitCode, err
	}
	return -1, nil
}

func Evaluate(completeCmd ast.CompleteCommand, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	// TODO: Handle separator (job control).
	if completeCmd.Separator == lexer.TokAmpersand {
		panic("job control not implemented")
	}
	return evaluateList(completeCmd.List, stdin, stdout, stderr)
}
