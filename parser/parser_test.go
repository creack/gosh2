package parser

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.creack.net/gosh2/ast"
	"go.creack.net/gosh2/lexer"
)

func mkCmd(input string) ast.SimpleCommand {
	parts := strings.Split(input, " ")
	if len(parts) == 1 {
		return ast.SimpleCommand{Name: input}
	}
	return ast.SimpleCommand{
		Name: parts[0],
		Suffix: ast.CmdSuffix{
			Words: parts[1:],
		},
	}
}

func TestParserSimple(t *testing.T) {
	// Create a lexer with some test input.
	lex := lexer.New(strings.NewReader("ls -l\n"))

	// Parse the input.
	prog := Parse(lex)

	cmd := mkCmd("ls -l")
	expect := ast.Program{Commands: []ast.CompleteCommand{{
		List: ast.List{AndOrs: []ast.AndOr{{
			Pipelines: []ast.Pipeline{{
				Commands: []ast.Command{cmd},
				Negated:  false,
			}},
			Operators: nil,
		}}},
		Separator: 0,
	}}}

	require.Equal(t, expect, prog)
}

func TestParserBadRedirect(t *testing.T) {
	t.SkipNow()
	// Create a lexer with some test input.
	lex := lexer.New(strings.NewReader("ls -l > foo> 2> bar baz"))

	// Parse the input.
	prog := Parse(lex)

	_ = prog
}

func TestParserComplex(t *testing.T) {
	// All inputs are equivalent.
	for _, input := range []string{
		"5<bar ls -l | cat -e | cat < foo && ! echo ok >bar >bar1 2>baz 3>>buz || 4>f echo ko& printf hello\ntree;",
		"\n5< bar ls -l | \ncat -e | cat 0< foo &&\n ! echo ok 1>bar >bar1 2>baz 3>>buz ||4>f echo ko& printf hello\n\ntree;",
		"5< bar ls -l | cat -e | \n\n cat <foo  && ! \necho ok>bar >bar1 2>baz 3>>buz || \n4>f echo ko&printf hello\ntree;\n",
	} {
		// Create a lexer with some test input.
		lex := lexer.New(strings.NewReader(input))

		// Parse the input.
		prog := Parse(lex)

		var (
			cmdLs     = mkCmd("ls -l")
			cmdCatE   = mkCmd("cat -e")
			cmdCat    = mkCmd("cat")
			cmdEchoOk = mkCmd("echo ok")
			cmdEchoKo = mkCmd("echo ko")
			cmdPrintf = mkCmd("printf hello")
			cmdTree   = mkCmd("tree")
		)
		cmdLs.Prefix.Redirects = []ast.IORedirect{
			{Number: 5, Op: lexer.TokRedirectIn, Filename: "bar"},
		}
		cmdCat.Suffix.Redirects = []ast.IORedirect{
			{Number: 0, Op: lexer.TokRedirectIn, Filename: "foo"},
		}
		cmdEchoOk.Suffix.Redirects = []ast.IORedirect{
			{Number: 1, Op: lexer.TokRedirectOut, Filename: "bar"},
			{Number: 1, Op: lexer.TokRedirectOut, Filename: "bar1"},
			{Number: 2, Op: lexer.TokRedirectOut, Filename: "baz"},
			{Number: 3, Op: lexer.TokDoubleRedirectOut, Filename: "buz"},
		}
		cmdEchoKo.Prefix.Redirects = []ast.IORedirect{
			{Number: 4, Op: lexer.TokRedirectOut, Filename: "f"},
		}

		expect := ast.Program{
			Commands: []ast.CompleteCommand{{
				List: ast.List{
					AndOrs: []ast.AndOr{{
						Pipelines: []ast.Pipeline{
							{Commands: []ast.Command{cmdLs, cmdCatE, cmdCat}},
							{Commands: []ast.Command{cmdEchoOk}, Negated: true},
							{Commands: []ast.Command{cmdEchoKo}},
						},
						Operators: []lexer.TokenType{lexer.TokLogicalAnd, lexer.TokLogicalOr},
					}, {
						Pipelines: []ast.Pipeline{
							{Commands: []ast.Command{cmdPrintf}},
						},
					}},
					Separators: []lexer.TokenType{lexer.TokAmpersand},
				},
				Separator: 0,
			}, {
				List: ast.List{
					AndOrs: []ast.AndOr{{
						Pipelines: []ast.Pipeline{
							{Commands: []ast.Command{cmdTree}},
						},
					}},
				},
				Separator: lexer.TokSemicolon,
			}},
		}

		require.Equal(t, expect, prog, "Unexpecte result for %q\n", input)
	}
}

func TestParseStream(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		inR, inW := io.Pipe()
		defer func() { _ = inR.Close() }() // Best effort.
		defer func() { _ = inW.Close() }() // Best effort.

		p := New(inR)
		for range 3 {
			go fmt.Fprintf(inW, "ls\n")

			cmd := p.NextCompleteCommand()

			require.NotNil(t, cmd)
			require.Len(t, cmd.List.AndOrs, 1)
			require.Len(t, cmd.List.AndOrs[0].Pipelines, 1)
			require.Len(t, cmd.List.AndOrs[0].Pipelines[0].Commands, 1)
			simpleCmd, ok := cmd.List.AndOrs[0].Pipelines[0].Commands[0].(ast.SimpleCommand)
			require.True(t, ok)
			require.Equal(t, "ls", simpleCmd.Name)
		}
	}()

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	select {
	case <-done:
	case <-timer.C:
		t.Fatal("timeout")
	}
}
