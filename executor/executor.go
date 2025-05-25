package executor

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"go.creack.net/gosh2/ast"
	"go.creack.net/gosh2/lexer"
)

func evaluateSimpleCommand(scmd *ast.SimpleCommand, stdin io.Reader, stderr io.Writer) (CmdIO, error) {
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

func evaluateCompoundCommand(compCmd *ast.CompoundCommandWrap, stdin io.Reader, stderr io.Writer) (CmdIO, error) {
	switch compCmd := compCmd.CompoundCommand.(type) {
	case *ast.SubshellCommand:
		exCmd := exec.Command(os.Args[0], "-sub", "-c", compCmd.Right.Dump())
		exCmd.Stdin = stdin
		exCmd.Stderr = stderr
		exCmd.Env = os.Environ()
		return &CmdWrap{exCmd}, nil
	default:
		panic(fmt.Errorf("unsupported compound command type %T", compCmd))
	}
}

func evaluateCommand(cmd ast.Command, stdin io.Reader, stderr io.Writer) (CmdIO, error) {
	switch c := cmd.(type) {
	case *ast.SimpleCommand:
		return evaluateSimpleCommand(c, stdin, stderr)
	case *ast.CompoundCommandWrap:
		return evaluateCompoundCommand(c, stdin, stderr)
	default:
		panic(fmt.Errorf("unsupported command type %T", c))
	}
}

func evaluatePipelineSequence(seq *ast.PipelineSequence, cmds *[]CmdIO, stdin io.Reader, stdout, stderr io.Writer) (CmdIO, error) {
	if seq.Left != nil {
		nextExCmd, err := evaluatePipelineSequence(seq.Left, cmds, stdin, stdout, stderr)
		if err != nil {
			return nil, err
		}

		fmt.Printf("OUTPIPE %q\n", nextExCmd.(*CmdWrap).Cmd)
		if nextExCmd.GetStdout() == nil {
			outPipe, err := nextExCmd.StdoutPipe()
			if err != nil {
				return nil, fmt.Errorf("stdout pipe %q: %w", nextExCmd.GetPath(), err)
			}
			stdin = outPipe
		}
		stderr = nextExCmd.GetStderr()
	}

	exCmd, err := evaluateCommand(seq.Right, stdin, stderr)
	if err != nil {
		return nil, fmt.Errorf("evaluate command %q: %w", seq.Right.Dump(), err)
	}
	if exCmd != nil {
		*cmds = append(*cmds, exCmd)
	}
	if err := setupCommandIO(seq.Right, exCmd); err != nil {
		return nil, fmt.Errorf("setup cmd io %q: %w", exCmd.GetPath(), err)
	}

	return exCmd, nil
}

func evaluatePipeline(pipeline *ast.Pipeline, stdin io.Reader, stdout, stderr io.Writer) (int, bool, error) {
	var cmds []CmdIO

	lastCmd, err := evaluatePipelineSequence(pipeline.Right, &cmds, stdin, stdout, stderr)
	if err != nil {
		return -1, pipeline.Negated, fmt.Errorf("evaluate pipeline sequence %q: %w", pipeline.Right.Dump(), err)
	}
	if lastCmd.GetStdout() == nil {
		lastCmd.SetStdout(stdout)
	}

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
