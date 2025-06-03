package parser

import (
	"fmt"
	"io"
	"os"
	"strings"

	"go.creack.net/gosh2/ast"
	"go.creack.net/gosh2/executor"
	"go.creack.net/gosh2/lexer"
)

// TokWorkd is an evaluted token type for words.
// TODO: Implement this.
const TokWord lexer.TokenType = lexer.FinalToken + iota + 1

type parser struct {
	lex *lexer.Lexer

	prevToken lexer.Token
	curToken  lexer.Token

	peekToken *lexer.Token // Buffer.

	// TODO: Reconsider this. Not a fan of having execution related fields in the parser itself.
	stderr io.Writer // Stderr for command substitution.
}

type Parser interface {
	NextCompleteCommand() *ast.CompleteCommand
}

func newParser(lex *lexer.Lexer, stderr io.Writer) *parser {
	if stderr == nil {
		stderr = os.Stderr
	}
	return &parser{
		lex:    lex,
		stderr: stderr,
	}
}

func New(r io.Reader, stderr io.Writer) Parser {
	return newParser(lexer.New(r), stderr)
}

func Parse(lex *lexer.Lexer, stderr io.Writer) ast.Program {
	var cmds []ast.CompleteCommand

	p := newParser(lex, stderr)
	for {
		cmd := p.NextCompleteCommand()
		if cmd == nil {
			break
		}
		cmds = append(cmds, *cmd)
	}

	return ast.Program{Commands: cmds}
}

func RunSubshell(argv []string, exitFn func(int), stdin io.Reader, stdout, stderr io.Writer) bool {
	// args[0] -sub -c 'cmd'
	if len(argv) != 4 || argv[1] != "-sub" || argv[2] != "-c" {
		return false
	}
	exitCode, err := Run(strings.NewReader(argv[3]), stdin, stdout, stderr)
	if err != nil {
		exitFn(1)
		return true
	}
	exitFn(exitCode)
	return true
}

func Run(input, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	p := New(input, stderr)

	var lastExitCode int
	for {
		cmd := p.NextCompleteCommand()
		if cmd == nil {
			break
		}
		exitCode, err := executor.Evaluate(*cmd, stdin, stdout, stderr)
		if err != nil {
			return exitCode, err
		}
		lastExitCode = exitCode
	}

	return lastExitCode, nil
}

func (p *parser) NextCompleteCommand() *ast.CompleteCommand {
	p.nextToken()
	p.ignoreWhitespaces()
	if p.curToken.Type == lexer.TokEOF || p.curToken.Type == lexer.TokError {
		return nil
	}
	return parseCompleteCommand(p)
}

func (p *parser) nextToken() lexer.Token {
	p.prevToken = p.curToken
	if p.peekToken != nil {
		p.curToken = *p.peekToken
		p.peekToken = nil
		p.curToken = p.aggregateTokens()
		return p.curToken
	}
	p.curToken = p.lex.NextToken()
	p.curToken = p.aggregateTokens()
	return p.curToken
}

func (p *parser) peek() lexer.Token {
	if p.peekToken != nil {
		return *p.peekToken
	}
	tok := p.lex.NextToken()
	p.peekToken = &tok
	return tok
}

func (p *parser) aggregateTokens() lexer.Token {
	tok := p.evalToken()

	words := []lexer.TokenType{
		lexer.TokIdentifier,
		lexer.TokNumber,
		lexer.TokSingleQuoteString,
		lexer.TokDoubleQuoteString,
		lexer.TokCmdSubstitution,
		lexer.TokBacktick,
	}

	if !tok.Type.IsOneOf(words...) {
		return tok
	}
	for p.peek().Type.IsOneOf(words...) {
		ntok := p.nextToken()
		tok.Value += ntok.Value
	}
	//	tok.Type = TokWord
	return tok
}

func (p *parser) evalToken() lexer.Token {
	tok := p.curToken

	switch tok.Type {
	case lexer.TokIdentifier:
		tok.Value = evalGlobing(tok.Value)
	case lexer.TokBacktick:
		tok = p.evalBacktick()
	case lexer.TokCmdSubstitution:
		tok = p.evalCommandSubstitution()
	case lexer.TokDoubleQuoteString:
		tok.Value = strings.ReplaceAll(tok.Value, "\\\"", "\"")
	}

	return tok
}

// expect checks if the current token is of the expected type.
func (p *parser) expect(kind ...lexer.TokenType) lexer.Token {
	if p.curToken.Type.IsOneOf(kind...) {
		return p.curToken
	}
	panic(fmt.Errorf("expected token %v but got %s (%s)", kind, p.curToken.Type, p.curToken))
}

// expectIdentifierStr checks if the current token is an identifier at large,
// i.e., it can be a raw identifier, a single/double quoted string or a number.
func (p *parser) expectIdentifierStr() lexer.Token {
	return p.expect(lexer.TokIdentifier, lexer.TokSingleQuoteString, lexer.TokDoubleQuoteString, lexer.TokNumber)
}

func (p *parser) ignoreWhitespaces() {
	for p.curToken.Type == lexer.TokWhitespace {
		p.nextToken()
	}
}

func (p *parser) ignoreNLWhitespaces() {
	for p.curToken.Type == lexer.TokWhitespace || p.curToken.Type == lexer.TokNewline {
		p.nextToken()
	}
}
