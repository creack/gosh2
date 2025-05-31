package lexer

import (
	"fmt"
	"slices"
)

// TokenType is the type of token.
type TokenType int

// Token types as constants.
const (
	TokError TokenType = iota
	TokEOF

	// Identifiers + literals.
	TokIdentifier
	TokSingleQuoteString
	TokDoubleQuoteString
	TokNumber
	TokVar

	// Redirects.
	TokRedirectLess           // '<'.
	TokRedirectGreat          // '>'.
	TokRedirectDoubleLess     // DLESS (<<).
	TokRedirectDoubleGreat    // DGREAT (>>).
	TokRedirectLessAnd        // LESSAND (<&).
	TokRedirectGreatAnd       // GREATAND (>&).
	TokRedirectLessGreat      // LESSGREAT (<>).
	TokRedirectDoubleLessDash // DLESSDASH (<<-).
	TokRedirectClobber        // CLOBBER (>|).

	// Operators.
	TokEquals
	TokBang
	TokAndIf
	TokOrIf

	// Delimiters.
	TokWhitespace
	TokNewline
	TokPipe
	TokComma
	TokSemicolon
	TokDoubleSemicolon
	TokAmpersand
	TokBacktick

	TokCmdSubstitution // $(
	TokParenLeft
	TokParenRight
	TokBraceLeft
	TokBraceRight
	TokBracketLeft
	TokBracketRight

	// End of tokens.
	FinalToken
)

var TokSeparatorOp = []TokenType{
	TokSemicolon,
	TokAmpersand,
}

var TokSeparator = append(TokSeparatorOp, TokNewline)

var TokAnyRedirect = []TokenType{
	TokRedirectLess,
	TokRedirectGreat,
	TokRedirectDoubleLess,
	TokRedirectDoubleGreat,
	TokRedirectLessAnd,
	TokRedirectGreatAnd,
	TokRedirectLessGreat,
	TokRedirectDoubleLessDash,
	TokRedirectClobber,
}

// String returns the string representation of the token type.
func (tt TokenType) String() string {
	return tokenTypeStrings[tt]
}

// Map of token types to their string representation for debugging.
var tokenTypeStrings = map[TokenType]string{
	TokError: "ERROR",
	TokEOF:   "EOF",

	TokIdentifier:        "IDENTIFIER",
	TokSingleQuoteString: "SINGLE_QUOTE_STRING",
	TokDoubleQuoteString: "DOUBLE_QUOTE_STRING",
	TokNumber:            "NUMBER",
	TokVar:               "VAR",

	TokRedirectLess:           "<",   // '<'.
	TokRedirectGreat:          ">",   // '>'.
	TokRedirectDoubleLess:     "<<",  // DLESS (<<).
	TokRedirectDoubleGreat:    ">>",  // DGREAT (>>).
	TokRedirectLessAnd:        "<&",  // LESSAND (<&).
	TokRedirectGreatAnd:       ">&",  // GREATAND (>&).
	TokRedirectLessGreat:      "<>",  // LESSGREAT (<>).
	TokRedirectDoubleLessDash: "<<-", // DLESSDASH (<<-).
	TokRedirectClobber:        ">|",  // CLOBBER (>|).

	TokEquals: "EQUALS",
	TokBang:   "BANG",
	TokAndIf:  "AND_IF",
	TokOrIf:   "OR_IF",

	TokWhitespace:      "WHITESPACE",
	TokNewline:         "NEWLINE",
	TokPipe:            "PIPE",
	TokComma:           "COMMA",
	TokSemicolon:       ";",
	TokDoubleSemicolon: "DOUBLE_SEMICOLON",
	TokAmpersand:       "&",
	TokBacktick:        "BACKTICK",

	TokCmdSubstitution: "CMD_SUBSTITUTION",
	TokParenLeft:       "PAREN_LEFT",
	TokParenRight:      "PAREN_RIGHT",
	TokBraceLeft:       "BRACE_LEFT",
	TokBraceRight:      "BRACE_RIGHT",
	TokBracketLeft:     "BRACKET_LEFT",
	TokBracketRight:    "BRACKET_RIGHT",
}

func (tt TokenType) IsOneOf(t ...TokenType) bool {
	return slices.Contains(t, tt)
}

// Token represents a lexical token in our shell.
type Token struct {
	Type  TokenType
	Value string

	pos  int
	line int
}

func (t Token) PrettyPrint() string {
	switch t.Type {
	case TokEOF:
		return ""
	case TokError:
		return t.errorString()

	case TokIdentifier, TokNumber:
		return t.Value
	case TokSingleQuoteString:
		return fmt.Sprintf("'%s'", t.Value)
	case TokDoubleQuoteString:
		return fmt.Sprintf("\"%s\"", t.Value)
	case TokVar:
		return fmt.Sprintf("$%s", t.Value)

	case TokBang:
		return "!"
	case TokEquals:
		return "="
	case TokAndIf:
		return "&&"
	case TokOrIf:
		return "||"

	case TokWhitespace:
		return " "
	case TokNewline:
		return "\n"
	case TokPipe:
		return "|"
	case TokComma:
		return ","
	case TokSemicolon:
		return ";"
	case TokDoubleSemicolon:
		return ";;"
	case TokAmpersand:
		return "&"
	case TokBacktick:
		return "`"

	case TokCmdSubstitution:
		return "$("
	case TokParenLeft:
		return "("
	case TokParenRight:
		return ")"
	case TokBraceLeft:
		return "{"
	case TokBraceRight:
		return "}"
	case TokBracketLeft:
		return "["
	case TokBracketRight:
		return "]"

	default:
		if t.Type >= TokRedirectLess && t.Type <= TokRedirectClobber {
			return fmt.Sprintf("%s%s", t.Value, t.Type)
		}
		return fmt.Sprintf("%s: %q", t.Type, t.Value)
	}
}

func (t Token) String() string {
	switch {
	case t.Type == TokEOF:
		return "EOF"
	case t.Type == TokError:
		return t.errorString()
	case len(t.Value) > 16:
		return fmt.Sprintf("%s[%d:%d]: %.16q", t.Type, t.line, t.pos, t.Value)
	}
	return fmt.Sprintf("%s[%d:%d]: %q", t.Type, t.line, t.pos, t.Value)
}

func (t Token) errorString() string {
	out := fmt.Sprintf("ERROR [%d:%d]: %s", t.line, t.pos, t.Value)
	return out
}
