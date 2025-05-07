package lexer

import "strings"

type stateFn func(*Lexer) stateFn

func lexText(l *Lexer) stateFn {
	l.acceptRun(" \t") // Consume leading whitespaces.
	l.ignore()         // Ignore leading whitespaces.
	if l.atEOF {
		return l.emit(TokEOF)
	}

	// List of runes that just advance one and emit a token.
	singles := map[rune]TokenType{
		'\n': TokNewline,
		'(':  TokParenLeft,
		')':  TokParenRight,
		'[':  TokBracketLeft,
		']':  TokBracketRight,
		'{':  TokBraceLeft,
		'}':  TokBraceRight,
		';':  TokSemicolon,
		'&':  TokAmpersand,
		'=':  TokEquals,
		',':  TokComma,
		'!':  TokBang,
	}

	switch r := l.peek(); {
	case r == 0:
		return l.emit(TokEOF)
	case r == '"', r == '\'':
		return lexString(r)
	case r == '$':
		return lexDollar
	case r == '<':
		l.next()
		if l.peek() == '<' {
			l.next()
			tok := l.thisToken(TokDoubleRedirectIn)
			// TODO: Handle HEREDOC. Get the HEREDOC name, consume everthing until we find it.
			_ = tok
			panic("HEREDOC not implemented")
			// return l.emitToken(tok)
		}
		tok := l.thisToken(TokRedirectIn)
		tok.Value = "0" // No number means stdin (0).
		return l.emitToken(tok)
	case r == '>':
		l.next()
		if l.peek() == '>' {
			l.next()
			tok := l.thisToken(TokDoubleRedirectOut)
			tok.Value = "1" // No number means stdout (1).
			return l.emitToken(tok)
		}
		tok := l.thisToken(TokRedirectOut)
		tok.Value = "1" // No number means stdout (1).
		return l.emitToken(tok)
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
		val := l.input[l.start:l.pos]
		l.next()
		double := l.peek() == peeked
		if double {
			l.next()
		}
		tok := l.thisToken(0)
		tok.Value = val
		switch {
		case peeked == '>' && !double:
			tok.Type = TokRedirectOut
		case peeked == '>' && double:
			tok.Type = TokDoubleRedirectOut
		case peeked == '<' && !double:
			tok.Type = TokRedirectIn
		case peeked == '<' && double:
			tok.Type = TokDoubleRedirectIn
		}
		return l.emitToken(tok)
	}
	l.acceptRun(digits)
	if l.peek() == '.' {
		l.next()
		l.acceptRun(digits)
	}
	return l.emit(TokIdentifier)
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
