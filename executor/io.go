package executor

import (
	"fmt"
	"os"

	"go.creack.net/gosh2/ast"
	"go.creack.net/gosh2/lexer"
)

func setupCommandIO(aCmd ast.Command, cmd CmdIO) error {
	for _, elem := range aCmd.IORedirects() {
		var openFlags int
		var file *os.File

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
				defer func() { _ = w.Close() }() // Best effort.
				fmt.Fprint(w, elem.IOFile.Filename)
			}()
			file = r
		default:
			return fmt.Errorf("unsupported redirect %q", elem.IOFile.Operator)
		}

		if file == nil && elem.IOFile.Filename != "" {
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
			file = f
		} else if elem.IOFile.ToNumber != nil {
			switch *elem.IOFile.ToNumber {
			case 0:
				file, _ = cmd.GetStdin().(*os.File)
			case 1:
				file, _ = cmd.GetStdout().(*os.File)
				if cmd.GetStdout() == nil {
					file = os.NewFile(uintptr(1), "stdout")
				}
			case 2:
				file, _ = cmd.GetStderr().(*os.File)
			default:
				file = cmd.GetExtraFD(*elem.IOFile.ToNumber)
			}
			fmt.Printf("------- %d -> %d\n", elem.Number, *elem.IOFile.ToNumber)
			if file == nil {
				return fmt.Errorf("bad file descriptor %d\n", *elem.IOFile.ToNumber)
			}
		} else if file == nil {
			return fmt.Errorf("missing filename or fd for %q", elem.IOFile.Operator)
		}

		switch elem.Number {
		case 0:
			cmd.SetStdin(file)
		case 1:
			cmd.SetStdout(file)
			fmt.Printf("1------- %d -> %d\n", elem.Number, 1)
			// Case for `>& filename`, redirect both stdout and stderr to the file.
			if elem.IOFile.Operator == lexer.TokRedirectGreatAnd && elem.IOFile.Filename != "" {
				cmd.SetStderr(file)
			}
		case 2:
			fmt.Printf("2------- %d -> %d\n", elem.Number, 2)
			cmd.SetStderr(file)
		default:
			cmd.SetExtraFD(elem.Number, file)
		}
	}
	return nil
}
