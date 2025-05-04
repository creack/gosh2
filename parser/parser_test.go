package parser

import (
	"testing"

	"github.com/sanity-io/litter"
	"go.creack.net/gosh2/lexer"
)

func TestParser(t *testing.T) {
	// Create a lexer with some test input.
	lex := lexer.NewLexer("1 | 2 > 3")

	// Parse the input.
	block := Parse(lex)

	litter.Dump(block)
}
