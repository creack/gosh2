// Package lexer provides a simple lexical analyzer for a shell language.
package lexer

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const variableChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_"
const identifiderChars = variableChars + ".-+*%/?"

type Lexer struct {
	reader *bufio.Reader

	input string // The current input string.

	curToken Token

	atEOF bool

	pos         int // Current position in input.
	line        int // Current line in input.
	linePos     int // Position of the current token in the line.
	prevLineLen int

	start     int // Position of the start of the current token.
	startLine int // Line where the current token started.
}

// New creates a new Lexer for the given input.
// This is just a placeholder for now.
func New(input io.Reader) *Lexer {
	return &Lexer{
		reader:    bufio.NewReader(input),
		line:      1,
		startLine: 1,
	}
}

func (l *Lexer) NextToken() Token {
	l.curToken = Token{Type: TokEOF, Value: "EOF", pos: l.pos, line: l.line}
	if l.atEOF {
		return l.curToken
	}
	state := lexText
	for {
		state = state(l)
		if state == nil {
			// fmt.Printf("LEXER: %s\n", l.curToken)
			// time.Sleep(1e9)
			return l.curToken
		}
	}
}

func (l *Lexer) next() rune {
	if l.atEOF {
		return 0
	}
	r, n, err := l.reader.ReadRune()
	if err != nil {
		if errors.Is(err, io.EOF) {
			l.atEOF = true
			return 0
		}
		panic(fmt.Errorf("read rune: %w", err))
	}
	l.input += string(r)
	l.pos += n
	l.linePos += n
	if r == '\n' {
		l.line++
		l.prevLineLen = l.linePos
		l.linePos = 0
	}
	return r
}

func (l *Lexer) backup() {
	// If we reached eof, we can't back up.
	// If we are at the beginning of the input, we can't back up.
	if l.atEOF || l.pos == 0 {
		return
	}
	if err := l.reader.UnreadRune(); err != nil {
		panic(fmt.Errorf("unread rune: %w", err))
	}
	r, n := utf8.DecodeLastRuneInString(l.input[:l.pos])
	l.pos -= n
	l.input = l.input[:l.pos]
	l.linePos -= n
	if r == '\n' {
		l.line--
		l.linePos = l.prevLineLen
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
		Type:  TokError,
		Value: fmt.Sprintf(format, args...),
		pos:   l.linePos,
		line:  l.line,
	}
	l.start = 0
	l.pos = 0
	l.atEOF = true
	l.reader = nil
	return nil
}
