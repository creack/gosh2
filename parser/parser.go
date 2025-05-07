package parser

import (
	"fmt"
	"slices"

	"go.creack.net/gosh2/ast"
	"go.creack.net/gosh2/lexer"
)

type parser struct {
	lex *lexer.Lexer

	prevToken lexer.Token
	curToken  lexer.Token
	peekToken lexer.Token
}

func newParser(lex *lexer.Lexer) *parser {
	p := &parser{
		lex: lex,
	}
	// Preload the peek token.
	p.nextToken()
	return p
}

func Parse(lex *lexer.Lexer) ast.Program {
	var cmds []ast.CompleteCommand

	p := newParser(lex)
	for {
		p.nextToken()
		for p.curToken.Type == lexer.TokNewline {
			p.nextToken()
		}
		if p.curToken.Type == lexer.TokEOF || p.curToken.Type == lexer.TokError {
			break
		}
		cmds = append(cmds, parseCompleteCommand(p))
	}

	return ast.Program{Commands: cmds}
}

func (p *parser) nextToken() lexer.Token {
	p.prevToken = p.curToken
	p.curToken = p.peekToken
	p.peekToken = p.lex.NextToken()
	return p.curToken
}

// expect checks if the current token is of the expected type.
func (p *parser) expect(kind ...lexer.TokenType) lexer.Token {
	if slices.Contains(kind, p.curToken.Type) {
		curToken := p.curToken
		return curToken
	}
	panic(fmt.Errorf("expected token %v but got %s (%s)", kind, p.curToken.Type, p.curToken))
}

func (p *parser) ignoreNewlines() {
	for p.curToken.Type == lexer.TokNewline {
		p.nextToken()
	}
}
