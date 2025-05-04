package parser

import (
	"testing"

	"github.com/kr/pretty"
	"go.creack.net/gosh2/lexer"
)

func TestParser(t *testing.T) {
	// Create a lexer with some test input.
	lex := lexer.NewLexer("bar=foo * 10 + (45 / 10 - -5)")

	// Parse the input.
	block := Parse(lex)

	pretty.Println(block)
}
