package lexer

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

// Helper function to test the lexer
func testLexer(t *testing.T, input string, expectedTokens []Token) {
	t.Helper()

	l := New(io.NopCloser(strings.NewReader(input)))
	var tokens []Token
	for {
		tok := l.NextToken()
		tokens = append(tokens, tok)
		if tok.Type == TokEOF {
			break
		}
	}
	if len(tokens) != len(expectedTokens) {
		t.Fatalf("Expected %d tokens, got %d", len(expectedTokens), len(tokens))
	}
	for i, expectedToken := range expectedTokens {
		token := tokens[i]

		if token.Type != expectedToken.Type {
			t.Fatalf("tests[%d] - wrong type. expected=%q (%s), got=%q (%s)",
				i, expectedToken.Type, expectedToken, token.Type, token)
		}

		if token.Value != expectedToken.Value {
			t.Fatalf("tests[%d] - wrong value. expected=%q (%s), got=%q (%s)",
				i, expectedToken.Value, expectedToken, token.Value, token)
		}
	}
}

func TestTokenTypeString(t *testing.T) {
	if len(tokenTypeStrings) != int(FinalToken) {
		t.Fatalf("Expected %d token types in tokenTypeStrings, got %d", FinalToken, len(tokenTypeStrings))
	}
}

func TestLexerSingleCommand(t *testing.T) {
	input := "ls"
	expectedTokens := []Token{
		{Type: TokIdentifier, Value: "ls"},
		{Type: TokEOF, Value: ""},
	}

	testLexer(t, input, expectedTokens)
}

func TestLexerBasicCommand(t *testing.T) {
	input := "ls -la"
	expectedTokens := []Token{
		{Type: TokIdentifier, Value: "ls"},
		{Type: TokIdentifier, Value: "-la"},
		{Type: TokEOF, Value: ""},
	}

	testLexer(t, input, expectedTokens)
}

func TestLexerCommandWithArgs(t *testing.T) {
	input := "cp file1.txt file2.txt"
	expectedTokens := []Token{
		{Type: TokIdentifier, Value: "cp"},
		{Type: TokIdentifier, Value: "file1.txt"},
		{Type: TokIdentifier, Value: "file2.txt"},
		{Type: TokEOF, Value: ""},
	}

	testLexer(t, input, expectedTokens)
}

func TestLexerPipe(t *testing.T) {
	input := "ls -l | grep .txt"
	expectedTokens := []Token{
		{Type: TokIdentifier, Value: "ls"},
		{Type: TokIdentifier, Value: "-l"},
		{Type: TokPipe, Value: "|"},
		{Type: TokIdentifier, Value: "grep"},
		{Type: TokIdentifier, Value: ".txt"},
		{Type: TokEOF, Value: ""},
	}

	testLexer(t, input, expectedTokens)
}

func TestLexerRedirection(t *testing.T) {
	input := "echo hello > output.txt"
	expectedTokens := []Token{
		{Type: TokIdentifier, Value: "echo"},
		{Type: TokIdentifier, Value: "hello"},
		{Type: TokRedirectOut, Value: "1"},
		{Type: TokIdentifier, Value: "output.txt"},
		{Type: TokEOF, Value: ""},
	}

	testLexer(t, input, expectedTokens)
}

func TestLexerQuotedStrings(t *testing.T) {
	input := `echo "This is a quoted string" 'And so is this'`
	expectedTokens := []Token{
		{Type: TokIdentifier, Value: "echo"},
		{Type: TokString, Value: "\"This is a quoted string\""},
		{Type: TokString, Value: "'And so is this'"},
		{Type: TokEOF, Value: ""},
	}

	testLexer(t, input, expectedTokens)
}

func TestLexerEnvVariables(t *testing.T) {
	input := "echo $HOME"
	expectedTokens := []Token{
		{Type: TokIdentifier, Value: "echo"},
		{Type: TokVar, Value: "$HOME"},
		{Type: TokEOF, Value: ""},
	}

	testLexer(t, input, expectedTokens)
}

func TestLexerCommandSubstitution(t *testing.T) {
	input := "echo $(ls -l)"
	expectedTokens := []Token{
		{Type: TokIdentifier, Value: "echo"},
		{Type: TokParenLeft, Value: "$("},
		{Type: TokIdentifier, Value: "ls"},
		{Type: TokIdentifier, Value: "-l"},
		{Type: TokParenRight, Value: ")"},
		{Type: TokEOF, Value: ""},
	}

	testLexer(t, input, expectedTokens)
}

func TestLexerComplexCommand(t *testing.T) {
	input := `find $HOME -name "*.go" | xargs grep "func main" > results.txt 2> errors.log &`
	expectedTokens := []Token{
		{Type: TokIdentifier, Value: "find"},
		{Type: TokVar, Value: "$HOME"},
		{Type: TokIdentifier, Value: "-name"},
		{Type: TokString, Value: "\"*.go\""},
		{Type: TokPipe, Value: "|"},
		{Type: TokIdentifier, Value: "xargs"},
		{Type: TokIdentifier, Value: "grep"},
		{Type: TokString, Value: "\"func main\""},
		{Type: TokRedirectOut, Value: "1"},
		{Type: TokIdentifier, Value: "results.txt"},
		{Type: TokRedirectOut, Value: "2"},
		{Type: TokIdentifier, Value: "errors.log"},
		{Type: TokAmpersand, Value: "&"},
		{Type: TokEOF, Value: ""},
	}

	testLexer(t, input, expectedTokens)
}

func TestLexerBraces(t *testing.T) {
	input := "if [ -f file.txt ]; then { echo \"File exists\"; }"
	expectedTokens := []Token{
		{Type: TokIdentifier, Value: "if"},
		{Type: TokBracketLeft, Value: "["},
		{Type: TokIdentifier, Value: "-f"},
		{Type: TokIdentifier, Value: "file.txt"},
		{Type: TokBracketRight, Value: "]"},
		{Type: TokSemicolon, Value: ";"},
		{Type: TokIdentifier, Value: "then"},
		{Type: TokBraceLeft, Value: "{"},
		{Type: TokIdentifier, Value: "echo"},
		{Type: TokString, Value: "\"File exists\""},
		{Type: TokSemicolon, Value: ";"},
		{Type: TokBraceRight, Value: "}"},
		{Type: TokEOF, Value: ""},
	}

	testLexer(t, input, expectedTokens)
}

func TestLexerBracketsForConditions(t *testing.T) {
	input := "[ $count -eq 10 ] && echo \"Count is 10\""
	expectedTokens := []Token{
		{Type: TokBracketLeft, Value: "["},
		{Type: TokVar, Value: "$count"},
		{Type: TokIdentifier, Value: "-eq"},
		{Type: TokIdentifier, Value: "10"},
		{Type: TokBracketRight, Value: "]"},
		{Type: TokLogicalAnd, Value: "&&"},
		{Type: TokIdentifier, Value: "echo"},
		{Type: TokString, Value: "\"Count is 10\""},
		{Type: TokEOF, Value: ""},
	}

	testLexer(t, input, expectedTokens)
}

func TestLexerVariableDeclaration(t *testing.T) {
	input := "name=\"John Doe\"\necho $name"
	expectedTokens := []Token{
		{Type: TokIdentifier, Value: "name"},
		{Type: TokEquals, Value: "="},
		{Type: TokString, Value: "\"John Doe\""},
		{Type: TokNewline, Value: "\n"},
		{Type: TokIdentifier, Value: "echo"},
		{Type: TokVar, Value: "$name"},
		{Type: TokEOF, Value: ""},
	}

	testLexer(t, input, expectedTokens)
}

func TestLexerBraceExpansion(t *testing.T) {
	input := "touch file{1,2,3}.txt"
	expectedTokens := []Token{
		{Type: TokIdentifier, Value: "touch"},
		{Type: TokIdentifier, Value: "file"},
		{Type: TokBraceLeft, Value: "{"},
		{Type: TokIdentifier, Value: "1"},
		{Type: TokComma, Value: ","},
		{Type: TokIdentifier, Value: "2"},
		{Type: TokComma, Value: ","},
		{Type: TokIdentifier, Value: "3"},
		{Type: TokBraceRight, Value: "}"},
		{Type: TokIdentifier, Value: ".txt"},
		{Type: TokEOF, Value: ""},
	}

	testLexer(t, input, expectedTokens)
}

func TestLexerVariableParameterExpansion(t *testing.T) {
	input := "echo ${name:-default}"
	expectedTokens := []Token{
		{Type: TokIdentifier, Value: "echo"},
		{Type: TokBraceLeft, Value: "${"},
		{Type: TokIdentifier, Value: "name"},
		{Type: TokIdentifier, Value: ":-default"},
		{Type: TokBraceRight, Value: "}"},
		{Type: TokEOF, Value: ""},
	}

	testLexer(t, input, expectedTokens)
}

func TestLexerArrayDeclaration(t *testing.T) {
	input := "files=(file1.txt file2.txt \"file with spaces.txt\")"
	expectedTokens := []Token{
		{Type: TokIdentifier, Value: "files"},
		{Type: TokEquals, Value: "="},
		{Type: TokParenLeft, Value: "("},
		{Type: TokIdentifier, Value: "file1.txt"},
		{Type: TokIdentifier, Value: "file2.txt"},
		{Type: TokString, Value: "\"file with spaces.txt\""},
		{Type: TokParenRight, Value: ")"},
		{Type: TokEOF, Value: ""},
	}

	testLexer(t, input, expectedTokens)
}

func TestLexerArrayAccess(t *testing.T) {
	input := "echo ${files[0]}"
	expectedTokens := []Token{
		{Type: TokIdentifier, Value: "echo"},
		{Type: TokBraceLeft, Value: "${"},
		{Type: TokIdentifier, Value: "files"},
		{Type: TokBracketLeft, Value: "["},
		{Type: TokIdentifier, Value: "0"},
		{Type: TokBracketRight, Value: "]"},
		{Type: TokBraceRight, Value: "}"},
		{Type: TokEOF, Value: ""},
	}

	testLexer(t, input, expectedTokens)
}

func TestLexerErrorCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Token
	}{
		{
			name:  "Unclosed double quotes",
			input: `echo "This string is not closed`,
			expected: []Token{
				{Type: TokIdentifier, Value: "echo"},
				{Type: TokError, Value: "unclosed double quote"},
				{Type: TokEOF, Value: ""},
			},
		},
		{
			name:  "Unclosed single quotes",
			input: `echo 'Single quoted string without closure`,
			expected: []Token{
				{Type: TokIdentifier, Value: "echo"},
				{Type: TokError, Value: "unclosed single quote"},
				{Type: TokEOF, Value: ""},
			},
		},
		{
			name:  "Empty command",
			input: "",
			expected: []Token{
				{Type: TokEOF, Value: ""},
			},
		},
		{
			name:  "Only whitespace",
			input: "   \t   \n   ",
			expected: []Token{
				{Type: TokNewline, Value: "\n"},
				{Type: TokEOF, Value: ""},
			},
		},
		{
			name:  "Command with only flags",
			input: "-l -a",
			expected: []Token{
				{Type: TokIdentifier, Value: "-l"},
				{Type: TokIdentifier, Value: "-a"},
				{Type: TokEOF, Value: ""},
			},
		},
		{
			name:  "Escaped quotes in string",
			input: `echo "String with \"escaped quotes\""`,
			expected: []Token{
				{Type: TokIdentifier, Value: "echo"},
				{Type: TokString, Value: "\"String with \\\"escaped quotes\\\"\""},
				{Type: TokEOF, Value: ""},
			},
		},
		{
			name:  "Mixed quotes",
			input: `echo "outer 'inner' quotes"`,
			expected: []Token{
				{Type: TokIdentifier, Value: "echo"},
				{Type: TokString, Value: "\"outer 'inner' quotes\""},
				{Type: TokEOF, Value: ""},
			},
		},
		{
			name:  "Empty quotes",
			input: `echo "" ''`,
			expected: []Token{
				{Type: TokIdentifier, Value: "echo"},
				{Type: TokString, Value: "\"\""},
				{Type: TokString, Value: "''"},
				{Type: TokEOF, Value: ""},
			},
		},
		{
			name:  "Multiple spaces between tokens",
			input: "ls    -l     file.txt",
			expected: []Token{
				{Type: TokIdentifier, Value: "ls"},
				{Type: TokIdentifier, Value: "-l"},
				{Type: TokIdentifier, Value: "file.txt"},
				{Type: TokEOF, Value: ""},
			},
		},
		{
			name:  "Flag with no space",
			input: "ls-l",
			expected: []Token{
				{Type: TokIdentifier, Value: "ls-l"},
				{Type: TokEOF, Value: ""},
			},
		},
		{
			name:  "Long option format",
			input: "ls --long-format file.txt",
			expected: []Token{
				{Type: TokIdentifier, Value: "ls"},
				{Type: TokIdentifier, Value: "--long-format"},
				{Type: TokIdentifier, Value: "file.txt"},
				{Type: TokEOF, Value: ""},
			},
		},
		{
			name:  "Flag combined with argument",
			input: "ls -lfile.txt",
			expected: []Token{
				{Type: TokIdentifier, Value: "ls"},
				{Type: TokIdentifier, Value: "-lfile.txt"},
				{Type: TokEOF, Value: ""},
			},
		},
		{
			name:  "Special characters in command",
			input: "!@#$%^&*()",
			expected: []Token{
				{Type: TokBang, Value: "!"},
				{Type: TokError, Value: "unexpected character: '@'"},
				{Type: TokEOF, Value: ""},
			},
		},
		{
			name:  "Multiple lines with newlines",
			input: "echo hello\necho world",
			expected: []Token{
				{Type: TokIdentifier, Value: "echo"},
				{Type: TokIdentifier, Value: "hello"},
				{Type: TokNewline, Value: "\n"},
				{Type: TokIdentifier, Value: "echo"},
				{Type: TokIdentifier, Value: "world"},
				{Type: TokEOF, Value: ""},
			},
		},
		{
			name:  "Tab separated tokens",
			input: "cat\tfile.txt",
			expected: []Token{
				{Type: TokIdentifier, Value: "cat"},
				{Type: TokIdentifier, Value: "file.txt"},
				{Type: TokEOF, Value: ""},
			},
		},
		{
			name:  "Single dollar sign",
			input: "$",
			expected: []Token{
				{Type: TokIdentifier, Value: "$"},
				{Type: TokEOF, Value: ""},
			},
		},
		{
			name:  "Double dollar sign",
			input: "$$",
			expected: []Token{
				{Type: TokVar, Value: "$$"},
				{Type: TokEOF, Value: ""},
			},
		},
		{
			name:  "Double variable",
			input: "$a$b",
			expected: []Token{
				{Type: TokVar, Value: "$a"},
				{Type: TokVar, Value: "$b"},
				{Type: TokEOF, Value: ""},
			},
		},
		{
			name:  "Unterminated double variable",
			input: "$a$",
			expected: []Token{
				{Type: TokVar, Value: "$a"},
				{Type: TokIdentifier, Value: "$"},
				{Type: TokEOF, Value: ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testLexer(t, tt.input, tt.expected)
		})
	}
}

func TestErrorPos(t *testing.T) {
	t.SkipNow()
	input := "echo\nld  ? world"
	lex := New(io.NopCloser(strings.NewReader(input)))
	lex.NextToken()
	lex.NextToken()
	token := lex.NextToken()
	if token.Type != TokError {
		t.Errorf("expected TokError, got %s", token.Type)
	}
	fmt.Println()
	fmt.Println(token)
	fmt.Println()
	fmt.Printf("%q\n", input[token.pos:])
}
