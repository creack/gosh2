package executor

import (
	"fmt"
	"io"
	"os"

	"go.creack.net/gosh2/ast"
	"go.creack.net/gosh2/lexer"
)

func setupCommandIO(aCmd ast.Command, cmd CmdIO) error {
	for _, elem := range aCmd.IORedirects() {
		var openFlags int
		var in io.Reader
		var out io.Writer

		switch elem.IOFile.Operator {
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
				defer func() { _ = w.Close() }()    // Best effort.
				fmt.Fprint(w, elem.IOFile.Filename) // The content until HEREDOC is stored in Filename.
			}()
			in = r
		default:
			return fmt.Errorf("unsupported redirect %q", elem.IOFile.Operator)
		}

		if in == nil && elem.IOFile.Filename != "" {
			// Check for invalid case `echo hello 4>& foo`.
			// The `>&` redirect only support '1' (or empty, which defaults to 1)
			// when used with a target filename.
			if elem.IOFile.Operator == lexer.TokRedirectGreatAnd && elem.Number != 1 {
				return fmt.Errorf("ambiguous redirect %q", elem.IOFile.Operator)
			}
			f, err := os.OpenFile(elem.IOFile.Filename, openFlags, 0o644)
			if err != nil {
				return fmt.Errorf("openfile %q: %w", elem.IOFile.Filename, err)
			}
			if elem.Number == 0 {
				in = f
			} else {
				out = f
			}
		} else if elem.IOFile.ToNumber != nil {
			switch *elem.IOFile.ToNumber {
			case 0:
				in = cmd.GetStdin()
			case 1:
				out = cmd.GetStdout()
				// 	if cmd.GetStdout() == nil {
				// 		out = os.NewFile(uintptr(1), "stdout")
				// 	}
			case 2:
				out = cmd.GetStderr()
			default:
				out = cmd.GetExtraFD(*elem.IOFile.ToNumber)
			}
			if in == nil && out == nil {
				return fmt.Errorf("bad file descriptor2 %d\n", *elem.IOFile.ToNumber)
			}
		} else if in == nil && out == nil {
			return fmt.Errorf("missing filename or fd for %q", elem.IOFile.Operator)
		}

		switch elem.Number {
		case 0:
			cmd.SetStdin(in)
		case 1:
			// TODO: Check if this leaks fd when `1>&2` is used as stdout is already set from the StdoutPipe.
			cmd.SetStdout(out)

			// Case for `>& filename`, redirect both stdout and stderr to the file.
			if elem.IOFile.Operator == lexer.TokRedirectGreatAnd && elem.IOFile.Filename != "" {
				cmd.SetStderr(out)
			}
		case 2:
			cmd.SetStderr(out)
		default:
			f, ok := out.(*os.File)
			if !ok {
				return fmt.Errorf("unsupported file descriptor %d for %q: not a file (%T)", elem.Number, elem.IOFile.Operator, out)
			}
			cmd.SetExtraFD(elem.Number, f)
		}
	}
	return nil
}
