package ast

import "go.creack.net/gosh2/lexer"

type NumberExpr struct {
	Value float64
}

func (NumberExpr) expr() {}

type StringExpr struct {
	Value string
}

func (StringExpr) expr() {}

type SymbolExpr struct {
	Value string
}

func (SymbolExpr) expr() {}

type BinaryExpr struct {
	Left     Expr
	Operator lexer.Token
	Right    Expr
}

func (BinaryExpr) expr() {}
