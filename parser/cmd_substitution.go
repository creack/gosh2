package parser

import (
	"bytes"
	"errors"
	"log"
	"os"
	"os/exec"
	"strings"

	"go.creack.net/gosh2/lexer"
)

func runCommandSubstitution(input string) lexer.Token {
	cmd := exec.Command(os.Args[0], "-sub", "-c", input)

	buf := bytes.NewBuffer(nil)
	cmd.Stdin = nil
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		var e0 *exec.ExitError
		if !errors.As(err, &e0) {
			log.Printf("Run sub command error: %s.", err)
		}
	}

	return lexer.Token{
		Type:  lexer.TokIdentifier,
		Value: strings.ReplaceAll(strings.TrimRight(buf.String(), "\n"), "\n", " "),
	}
}

func (p *parser) evalBacktick() lexer.Token {
	var values []string

	p.expect(lexer.TokBacktick)
	p.curToken = p.lex.NextToken()
	for !p.curToken.Type.IsOneOf(lexer.TokEOF, lexer.TokError, lexer.TokBacktick) {
		values = append(values, p.curToken.Value)
		p.curToken = p.lex.NextToken()
	}
	p.expect(lexer.TokBacktick)

	str := strings.Join(values, "")
	return runCommandSubstitution(str)
}

func (p *parser) evalCommandSubstitution() lexer.Token {
	var values []string

	p.curToken = p.lex.NextToken()
	depth := 1
	for {
		if p.curToken.Type.IsOneOf(lexer.TokParenLeft, lexer.TokCmdSubstitution) {
			depth++
		}
		if p.curToken.Type == lexer.TokParenRight {
			depth--
		}
		if depth == 0 || p.curToken.Type.IsOneOf(lexer.TokEOF, lexer.TokError) {
			break
		}
		values = append(values, p.curToken.Value)
		p.curToken = p.lex.NextToken()
	}
	str := strings.Join(values, "")
	return runCommandSubstitution(str)
}
