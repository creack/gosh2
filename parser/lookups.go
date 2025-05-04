package parser

import (
	"go.creack.net/gosh2/ast"
	"go.creack.net/gosh2/lexer"
)

type bindingPower int

const (
	bpDefault bindingPower = iota
	bpAssignment
	bpRedirect
	bpPipe
	bpAdditive
	bpMultiplicative
	bpPrimary
)

type stmtHandler func(*parser) ast.Stmt
type nudHandler func(*parser) ast.Expr
type ledHandler func(*parser, ast.Expr, bindingPower) ast.Expr

type lookupTable[T any] map[lexer.TokenType]T

func (p *parser) led(kind lexer.TokenType, bp bindingPower, fn ledHandler) {
	if _, ok := p.ledLookupTable[kind]; ok {
		panic("duplicate led handler")
	}
	p.ledLookupTable[kind] = fn
	p.bindingPowerLookupTable[kind] = bp
}

func (p *parser) nud(kind lexer.TokenType, fn nudHandler) {
	if _, ok := p.nudLookupTable[kind]; ok {
		panic("duplicate nud handler")
	}
	p.nudLookupTable[kind] = fn
}

func (p *parser) stmt(kind lexer.TokenType, fn stmtHandler) {
	if _, ok := p.stmtLookupTable[kind]; ok {
		panic("duplicate stmt handler")
	}
	p.stmtLookupTable[kind] = fn
	p.bindingPowerLookupTable[kind] = bpDefault
}

func (p *parser) createTokenLookups() {
	// Assignment.
	p.led(lexer.TokEquals, bpAssignment, parseAssignmentExpr)

	p.led(lexer.TokRedirectOut, bpRedirect, parseBinaryExpr)
	p.led(lexer.TokPipe, bpPipe, parseBinaryExpr)

	// Additional & multiplicative.
	p.led(lexer.TokPlus, bpAdditive, parseBinaryExpr)
	p.led(lexer.TokDash, bpAdditive, parseBinaryExpr)
	p.led(lexer.TokMultiply, bpMultiplicative, parseBinaryExpr)
	p.led(lexer.TokModulo, bpMultiplicative, parseBinaryExpr)
	p.led(lexer.TokSlash, bpMultiplicative, parseBinaryExpr)

	// Literals & symbols.
	p.nud(lexer.TokNumber, parsePrimaryExpr)
	p.nud(lexer.TokString, parsePrimaryExpr)
	p.nud(lexer.TokIdentifier, parsePrimaryExpr)
	p.nud(lexer.TokParenLeft, parseGroupingExpr)
	p.nud(lexer.TokDash, parsePrefixExpr)
}
