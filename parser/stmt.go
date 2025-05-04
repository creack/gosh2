package parser

import (
	"go.creack.net/gosh2/ast"
	"go.creack.net/gosh2/lexer"
)

func parseStmt(p *parser) ast.Stmt {
	stmtFn, exists := p.stmtLookupTable[p.curToken.Type]
	if exists {
		return stmtFn(p)
	}

	expression := parseExpr(p, bpDefault)
	p.expect(lexer.TokSemicolon, lexer.TokNewline, lexer.TokEOF)

	return ast.ExpressionStmt{
		Expression: expression,
	}
}
