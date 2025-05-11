package executor

import (
	"fmt"
	"os"

	"go.creack.net/gosh2/ast"
	"go.creack.net/gosh2/lexer"
)

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
