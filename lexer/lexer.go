// Package lexer provides a simple lexical analyzer for a shell language.
package lexer

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const variableChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_"
const identifiderChars = variableChars + "."

type stateFn func(*Lexer) stateFn

// TokenType is the type of token.
type TokenType int

// Token types as constants.
const (
	TokIllegal TokenType = iota
	TokEOF

	// Identifiers + literals.
	TokIdentifier
	TokNumber
	TokString
	TokVar

	// Operators.
	TokPipe
	TokRedirectIn
	TokRedirectOut
	TokAppendOut
	TokRedirectErr
	TokBackground
	TokEquals
	TokPlus
	TokMultiply
	TokDash
	TokSlash
	TokModulo

	// Delimiters.
	TokNewline
	TokComma
	TokSemicolon
	TokQuoteDouble
	TokQuoteSingle
	TokParenLeft
	TokParenRight
	TokBraceLeft
	TokBraceRight
	TokBracketLeft
	TokBracketRight

	// End of tokens.
	FinalToken
)

// String returns the string representation of the token type.
func (tt TokenType) String() string {
	return tokenTypeStrings[tt]
}

// Map of token types to their string representation for debugging.
var tokenTypeStrings = map[TokenType]string{
	TokIllegal: "ILLEGAL",
	TokEOF:     "EOF",

	TokIdentifier: "IDENTIFIER",
	TokNumber:     "NUMBER",
	TokString:     "STRING",
	TokVar:        "VAR",

	TokPipe:        "PIPE",
	TokRedirectIn:  "REDIRECT_IN",
	TokRedirectOut: "REDIRECT_OUT",
	TokAppendOut:   "APPEND_OUT",
	TokRedirectErr: "REDIRECT_ERR",
	TokBackground:  "BACKGROUND",
	TokEquals:      "EQUALS",
	TokPlus:        "PLUS",
	TokMultiply:    "MULTIPLY",
	TokDash:        "DASH",
	TokSlash:       "SLASH",
	TokModulo:      "MODULO",

	TokNewline:      "NEWLINE",
	TokComma:        "COMMA",
	TokSemicolon:    "SEMICOLON",
	TokQuoteDouble:  "QUOTE_DOUBLE",
	TokQuoteSingle:  "QUOTE_SINGLE",
	TokParenLeft:    "PAREN_LEFT",
	TokParenRight:   "PAREN_RIGHT",
	TokBraceLeft:    "BRACE_LEFT",
	TokBraceRight:   "BRACE_RIGHT",
	TokBracketLeft:  "BRACKET_LEFT",
	TokBracketRight: "BRACKET_RIGHT",
}

// Token represents a lexical token in our shell.
type Token struct {
	Type  TokenType
	Value string

	pos  int
	line int
}

func (t Token) String() string {
	switch {
	case t.Type == TokEOF:
		return "EOF"
	case t.Type == TokIllegal:
		return fmt.Sprintf("ILLEGAL[%d:%d]: %q", t.line, t.pos, t.Value)
	case len(t.Value) > 10:
		return fmt.Sprintf("%s[%d:%d]: %.10q", t.Type, t.line, t.pos, t.Value)
	}
	return fmt.Sprintf("%s[%d:%d]: %q", t.Type, t.line, t.pos, t.Value)
}

type Lexer struct {
	input string

	curToken Token

	atEOF bool

	pos  int // Current position in input.
	line int // Current line in input.

	start     int // Position of the start of the current token.
	startLine int // Line where the current token started.
}

func (l *Lexer) next() rune {
	if l.pos >= len(l.input) {
		l.atEOF = true
		return 0
	}
	r, n := utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += n
	if r == '\n' {
		l.line++
	}
	return r
}

func (l *Lexer) backup() {
	// If we reached eof, we can't back up.
	// If we are at the beginning of the input, we can't back up.
	if l.atEOF || l.pos == 0 {
		return
	}
	r, n := utf8.DecodeLastRuneInString(l.input[:l.pos])
	l.pos -= n
	if r == '\n' {
		l.line--
	}
}

func (l *Lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

func (l *Lexer) accept(valid string) bool {
	if strings.ContainsRune(valid, l.next()) {
		return true
	}
	l.backup()
	return false
}

func (l *Lexer) acceptRun(valid string) bool {
	accepted := false
	for strings.ContainsRune(valid, l.next()) {
		accepted = true
	}
	l.backup()
	return accepted
}

func (l *Lexer) thisToken(tt TokenType) Token {
	t := Token{
		Type:  tt,
		Value: l.input[l.start:l.pos],
		pos:   l.pos,
		line:  l.line,
	}
	l.start = l.pos
	l.startLine = l.line
	return t
}

func (l *Lexer) emitToken(t Token) stateFn {
	l.curToken = t
	return nil
}

func (l *Lexer) emit(tt TokenType) stateFn {
	return l.emitToken(l.thisToken(tt))
}

func (l *Lexer) ignore() {
	l.line += strings.Count(l.input[l.start:l.pos], "\n")
	l.start = l.pos
	l.startLine = l.line
}

func (l *Lexer) errorf(format string, args ...any) stateFn {
	l.curToken = Token{
		Type:  TokIllegal,
		Value: fmt.Sprintf(format, args...),
		pos:   l.pos,
		line:  l.line,
	}
	l.start = 0
	l.pos = 0
	l.input = l.input[:0]
	return nil
}

func (l *Lexer) NextToken() Token {
	l.curToken = Token{Type: TokEOF, Value: "EOF", pos: l.pos, line: l.line}
	state := lexText
	for {
		state = state(l)
		if state == nil {
			fmt.Printf("LEXER: %s\n", l.curToken)
			return l.curToken
		}
	}
}

// NewLexer creates a new Lexer for the given input.
// This is just a placeholder for now.
func NewLexer(input string) *Lexer {
	l := &Lexer{
		input:     input,
		line:      1,
		startLine: 1,
	}
	return l
}

func lexText(l *Lexer) stateFn {
	l.acceptRun(" \t\n") // Consume leading whitespaces.
	l.ignore()           // Ignore leading whitespaces.
	if l.atEOF {
		return l.emit(TokEOF)
	}

	// List of runes that just advance one and emit a token.
	singles := map[rune]TokenType{
		'(': TokParenLeft,
		')': TokParenRight,
		'[': TokBracketLeft,
		']': TokBracketRight,
		'{': TokBraceLeft,
		'}': TokBraceRight,
		';': TokSemicolon,
		'&': TokBackground,
		'=': TokEquals,
		'|': TokPipe,
		'>': TokRedirectOut,
		'+': TokPlus,
		'-': TokDash,
		'*': TokMultiply,
		'/': TokSlash,
		'%': TokModulo,
		',': TokComma,
	}

	switch r := l.peek(); {
	case r == 0:
		return l.emit(TokEOF)
	case r == '"', r == '\'':
		return lexString(r)
	case r == '$':
		return lexDollar
	case r == '&':
		l.next()
		if l.peek() == '&' {
			l.next()
			return l.emit(TokIdentifier)
		}
		return l.emit(TokBackground)
	case r >= '0' && r <= '9':
		return lexNumber
	case r == ':':
		l.next()
		return lexIdentifier
	case strings.ContainsRune(identifiderChars, r):
		return lexIdentifier
	default:
		if tok, ok := singles[r]; ok {
			l.next()
			return l.emit(tok)
		}
		return l.errorf("unexpected character: %q", r)
	}
}

func lexNumber(l *Lexer) stateFn {
	const digits = "0123456789"
	l.accept(digits)
	if l.peek() == '>' {
		if l.input[l.start] != '2' {
			return l.errorf("unexpected character: %q", l.input[l.start])
		}
		l.next()
		return l.emit(TokRedirectErr)
	}
	l.acceptRun(digits)
	if l.peek() == '.' {
		l.next()
		l.acceptRun(digits)
	}
	return l.emit(TokNumber)
}

func lexDollar(l *Lexer) stateFn {
	l.accept("$")
	switch l.peek() {
	case '$':
		l.next()
		return l.emit(TokVar)
	case '(':
		l.next()
		return l.emit(TokParenLeft)
	case '{':
		l.next()
		return l.emit(TokBraceLeft)
	}
	if !l.acceptRun(variableChars) {
		// Case for lone '$', make it an identifier.
		return lexIdentifier
	}
	return l.emit(TokVar)
}

func lexString(kind rune) stateFn {
	return func(l *Lexer) stateFn {
		l.accept(string(kind))
		for {
			r := l.next()
			if r == 0 {
				if kind == '"' {
					return l.errorf("unclosed double quote")
				}
				return l.errorf("unclosed single quote")
			}
			if r == kind {
				break
			}
			if r == '\\' {
				l.next()
			}
		}
		return l.emit(TokString)
	}
}

func lexIdentifier(l *Lexer) stateFn {
	l.acceptRun(identifiderChars)
	return l.emit(TokIdentifier)
}
