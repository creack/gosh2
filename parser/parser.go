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

	stmtLookupTable         lookupTable[stmtHandler]
	nudLookupTable          lookupTable[nudHandler]
	ledLookupTable          lookupTable[ledHandler]
	bindingPowerLookupTable lookupTable[bindingPower]
}

func newParser(lex *lexer.Lexer) *parser {
	p := &parser{
		lex: lex,

		stmtLookupTable:         lookupTable[stmtHandler]{},
		nudLookupTable:          lookupTable[nudHandler]{},
		ledLookupTable:          lookupTable[ledHandler]{},
		bindingPowerLookupTable: lookupTable[bindingPower]{},
	}
	p.createTokenLookups()
	return p
}

func Parse(lex *lexer.Lexer) ast.BlockStmt {
	var stms []ast.Stmt

	p := newParser(lex)
	for p.curToken.Type != lexer.TokEOF {
		p.nextToken()

		stms = append(stms, parseStmt(p))
	}

	return ast.BlockStmt{
		Stmts: stms,
	}
}

func (p *parser) nextToken() lexer.Token {
	p.prevToken = p.curToken
	p.curToken = p.lex.NextToken()
	return p.curToken
}

func (p *parser) expect(kind ...lexer.TokenType) {
	if slices.Contains(kind, p.curToken.Type) {
		p.nextToken()
		return
	}
	panic(fmt.Errorf("expected token %v but got %v", kind, p.curToken.Type))
}
