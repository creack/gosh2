package lexer

import "strings"

type stateFn func(*Lexer) stateFn

func lexText(l *Lexer) stateFn {
	if l.atEOF {
		return l.emit(TokEOF)
	}

	// List of runes that just advance one and emit a token.
	singles := map[rune]TokenType{
		'\n': TokNewline,
		'(':  TokParenLeft,
		')':  TokParenRight,
		'&':  TokAmpersand,
		'=':  TokEquals,
		'!':  TokBang,
	}

	switch r := l.peek(); {
	case r == 0:
		return l.emit(TokEOF)
	case r == ' ' || r == '\t':
		l.acceptRun(" \t")
		return l.emit(TokWhitespace)
	case r == '\\':
		return lexIdentifier
	case r == '"', r == '\'':
		return lexString(r)
	case r == '$':
		return lexDollar
	case r == '>', r == '<':
		return lexRedirect
	case r == ';':
		l.next()
		if l.peek() == ';' {
			l.next()
			return l.emit(TokDoubleSemicolon)
		}
		return l.emit(TokSemicolon)
	case r == '|':
		l.next()
		if l.peek() == '|' {
			l.next()
			return l.emit(TokLogicalOr)
		}
		return l.emit(TokPipe)
	case r == '&':
		l.next()
		if l.peek() == '&' {
			l.next()
			return l.emit(TokLogicalAnd)
		}
		return l.emit(TokAmpersand)
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
	l.acceptRun(digits)
	if peeked := l.peek(); peeked == '>' || peeked == '<' {
		return lexRedirect
	}
	l.acceptRun(digits)
	if l.peek() == '.' {
		l.next()
		l.acceptRun(digits)
	}
	return l.emit(TokNumber)
}

func lexRedirect(l *Lexer) stateFn {
	peeked := l.peek()
	tok := l.thisToken(0)
	l.next()
	nextTok := l.peek()

	if tok.Value == "" {
		if peeked == '>' {
			tok.Value = "1"
		} else {
			tok.Value = "0"
		}
	}

	switch {
	case peeked == '>' && nextTok == '>':
		l.next()
		tok.Type = TokRedirectDoubleGreat
	case peeked == '>' && nextTok == '&':
		l.next()
		tok.Type = TokRedirectGreatAnd
	case peeked == '>' && nextTok == '|':
		l.next()
		tok.Type = TokRedirectClobber
	case peeked == '>':
		tok.Type = TokRedirectGreat

	case peeked == '<' && nextTok == '<':
		l.next()
		if l.peek() == '-' {
			l.next()
			tok.Type = TokRedirectDoubleLessDash
		} else {
			tok.Type = TokRedirectDoubleLess
		}
	case peeked == '<' && nextTok == '&':
		l.next()
		tok.Type = TokRedirectLessAnd
	case peeked == '<' && nextTok == '>':
		l.next()
		tok.Type = TokRedirectLessGreat
	case peeked == '<':
		tok.Type = TokRedirectLess
	}

	return l.emitToken(tok)
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
		l.ignore()
		for {
			r := l.next()
			if r == 0 {
				return l.errorf("unclosed %q", kind)
			}
			if r == kind {
				break
			}
			if kind == '"' && r == '\\' { // Single quote doesn't escape.
				l.next()
			}
		}
		tokType := TokSingleQuoteString
		if kind == '"' {
			tokType = TokDoubleQuoteString
		}
		tok := l.thisToken(tokType)
		tok.Value = strings.TrimSuffix(tok.Value, string(kind))
		return l.emitToken(tok)
	}
}

func lexIdentifier(l *Lexer) stateFn {
	l.acceptRun(identifiderChars)
	if l.peek() == '\\' {
		l.next() // Consume the backslash.
		l.next() // Consume the escaped character.
		return lexIdentifier
	}
	return l.emit(TokIdentifier)
}
