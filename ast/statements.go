package ast

type BlockStmt struct {
	Stmts []Stmt
}

func (BlockStmt) stmt() {}

type ExpressionStmt struct {
	Expression Expr
}

func (ExpressionStmt) stmt() {}
