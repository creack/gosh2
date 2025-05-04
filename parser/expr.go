package parser

import (
	"strconv"

	"go.creack.net/gosh2/ast"
	"go.creack.net/gosh2/lexer"
)

func parseExpr(p *parser, bp bindingPower) ast.Expr {
	// Parse the primary expression, always start with nud.
	nudFn, exists := p.nudLookupTable[p.curToken.Type]
	if !exists {
		// TODO: Handle errors properly.
		panic("nud handler not found")
	}
	left := nudFn(p)

	// While we have tokens with a higher binding power, parse them using led.
	for p.bindingPowerLookupTable[p.curToken.Type] > bp {
		ledFn, exists := p.ledLookupTable[p.curToken.Type]
		if !exists {
			// TODO: Handle errors properly.
			panic("led handler not found")
		}
		left = ledFn(p, left, p.bindingPowerLookupTable[p.curToken.Type])
	}

	return left
}

func parsePrimaryExpr(p *parser) ast.Expr {
	switch p.curToken.Type {
	case lexer.TokNumber:
		val := p.curToken.Value
		number, err := strconv.ParseFloat(val, 64)
		if err != nil {
			// TODO: Handle errors properly.
			panic("invalid number")
		}
		p.nextToken()
		return &ast.NumberExpr{
			Value: number,
		}
	case lexer.TokString:
		val := p.curToken.Value
		p.nextToken()
		return &ast.StringExpr{
			Value: val,
		}
	case lexer.TokIdentifier:
		val := p.curToken.Value
		p.nextToken()
		return &ast.SymbolExpr{
			Value: val,
		}
	default:
		// TODO: Handle errors properly.
		panic("unexpected token")
	}
}

func parseBinaryExpr(p *parser, left ast.Expr, bp bindingPower) ast.Expr {
	operator := p.curToken
	p.nextToken()
	right := parseExpr(p, bp)

	return ast.BinaryExpr{
		Left:     left,
		Operator: operator,
		Right:    right,
	}
}
