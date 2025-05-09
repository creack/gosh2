package parser

import (
	"fmt"
	"io"

	"go.creack.net/gosh2/ast"
	"go.creack.net/gosh2/lexer"
)

type parser struct {
	lex *lexer.Lexer

	prevToken lexer.Token
	curToken  lexer.Token
	// peekToken lexer.Token
}

type Parser interface {
	NextCompleteCommand() *ast.CompleteCommand
}

func newParser(lex *lexer.Lexer) *parser {
	return &parser{
		lex: lex,
	}
}

func New(r io.Reader) Parser {
	return newParser(lexer.New(r))
}

func Parse(lex *lexer.Lexer) ast.Program {
	var cmds []ast.CompleteCommand

	p := newParser(lex)
	for {
		cmd := p.NextCompleteCommand()
		if cmd == nil {
			break
		}
		cmds = append(cmds, *cmd)
	}

	return ast.Program{Commands: cmds}
}

func (p *parser) NextCompleteCommand() *ast.CompleteCommand {
	p.nextToken()
	p.ignoreWhitespaces()
	if p.curToken.Type == lexer.TokEOF || p.curToken.Type == lexer.TokError {
		return nil
	}
	completeCmd := parseCompleteCommand(p)
	return &completeCmd
}

func (p *parser) nextToken() lexer.Token {
	p.prevToken = p.curToken
	p.curToken = p.lex.NextToken()
	return p.curToken
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
	for p.curToken.Type.IsOneOf(lexer.TokNewline, lexer.TokWhitespace) {
		p.nextToken()
	}
}
