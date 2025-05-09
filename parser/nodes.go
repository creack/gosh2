package parser

import (
	"fmt"
	"strconv"

	"go.creack.net/gosh2/ast"
	"go.creack.net/gosh2/lexer"
)

func parseCompleteCommand(p *parser) ast.CompleteCommand {
	return *genParseCompleteCommand(p, &ast.CompleteCommand{}, parseList, nil)
}

type SetLister[T any] interface {
	SetList(list T)
	SetSeparator(sep lexer.TokenType)
}

func genParseCompleteCommand[T SetLister[L], L any](p *parser, ccmd T, hdlr func(*parser, []lexer.TokenType) L, endTokens []lexer.TokenType) T {
	p.ignoreWhitespaces()
	endTokens = append(endTokens, lexer.TokEOF, lexer.TokNewline)

	ccmd.SetList(hdlr(p, endTokens))
	if p.curToken.Type.IsOneOf(lexer.TokAmpersand, lexer.TokSemicolon) {
		ccmd.SetSeparator(p.curToken.Type)
		p.nextToken()
	}

	p.expect(endTokens...)
	return ccmd
}

func parseList(p *parser, endTokens []lexer.TokenType) ast.List {
	return *genParseList(p, &ast.List{}, endTokens)
}

type SetAndOrer interface {
	AppendAndOr(andOr ast.AndOr)
	AppendSeparator(sep lexer.TokenType)
}

func genParseList[T SetAndOrer](p *parser, list T, endTokens []lexer.TokenType) T {
	p.ignoreWhitespaces()

	for {
		andOr := parseAndOr(p, endTokens)
		list.AppendAndOr(andOr)

		if p.curToken.Type.IsOneOf(lexer.TokSemicolon, lexer.TokAmpersand) {
			p.nextToken()
			// A list cannot end with a separator, if there is one, it must be followed by a and_or.
			// If it is not, it is the end of the list.
			if p.curToken.Type.IsOneOf(endTokens...) {
				return list
			}
			list.AppendSeparator(p.prevToken.Type)
			continue
		}
		break
	}

	p.expect(endTokens...)
	return list
}

func parseAndOr(p *parser, endTokens []lexer.TokenType) ast.AndOr {
	p.ignoreWhitespaces()
	endTokens = append(endTokens, lexer.TokAmpersand, lexer.TokSemicolon, lexer.TokWhitespace)

	andOr := ast.AndOr{}
	for {
		pipeline := parsePipeline(p, endTokens)
		andOr.Pipelines = append(andOr.Pipelines, pipeline)
		if p.curToken.Type.IsOneOf(lexer.TokLogicalAnd, lexer.TokLogicalOr) {
			andOr.Operators = append(andOr.Operators, p.curToken.Type)
			p.nextToken()
			continue
		}
		break
	}

	p.expect(endTokens...)
	return andOr
}

func parsePipeline(p *parser, endTokens []lexer.TokenType) ast.Pipeline {
	p.ignoreWhitespaces()
	endTokens = append(endTokens, lexer.TokLogicalAnd, lexer.TokLogicalOr)

	pipeline := ast.Pipeline{}

	// Check for negation at the start of the pipeline.
	if p.curToken.Type == lexer.TokBang {
		pipeline.Negated = true
		p.nextToken()
	}

	// Parse the commands.
	for {
		switch p.curToken.Type {
		case lexer.TokIdentifier, lexer.TokSingleQuoteString, lexer.TokDoubleQuoteString, lexer.TokNumber:
			cmd := parseCommand(p, endTokens)
			pipeline.Commands = append(pipeline.Commands, cmd)
		case lexer.TokParenLeft:
			p.nextToken() // Consume the left parenthesis.
			cmd := parseSubshell(p)
			pipeline.Commands = append(pipeline.Commands, cmd)
		}
		// If the next token is a pipe, continue parsing.
		if p.curToken.Type == lexer.TokPipe {
			p.nextToken()
			p.ignoreWhitespaces()
			continue
		}

		// Otherwise, break the loop.
		break
	}

	p.expect(endTokens...)
	return pipeline
}

func parseSubshell(p *parser) ast.CompoundCommand {
	p.ignoreWhitespaces()
	compCmd := ast.CompoundCommand{
		Type: "subshell",
		Body: nil,
		// TODO: Handle redirects.
	}

	parseTerm := func(p *parser, endTokens []lexer.TokenType) ast.Term {
		return *genParseList(p, &ast.Term{}, endTokens)
	}
	parseCompoundList := func(p *parser) ast.CompoundList {
		return *genParseCompleteCommand(p, &ast.CompoundList{}, parseTerm, []lexer.TokenType{lexer.TokParenRight})
	}
	compCmd.Body = parseCompoundList(p)
	p.expect(lexer.TokParenRight)
	p.nextToken() // Consume the right parenthesis.
	return compCmd
}

func parseCommand(p *parser, endToken []lexer.TokenType) ast.SimpleCommand {
	p.ignoreWhitespaces()
	endToken = append(endToken, lexer.TokPipe)

	simpleCmd := ast.SimpleCommand{}

	// Handle prefixes.
	simpleCmd.Prefix = parseCommandPrefix(p)

	// Handle the command name.
	// TODO: Add support for `e"c"h'o' hello world`.
	simpleCmd.Name = p.expectIdentifierStr().Value
	p.nextToken()

	// Expect whitespace or end token.
	p.expect(endToken...)
	p.ignoreWhitespaces()

	// As long as we have a word, we can add it to the suffix.
	for p.curToken.Type.IsOneOf(lexer.TokIdentifier, lexer.TokSingleQuoteString, lexer.TokDoubleQuoteString, lexer.TokNumber) {
		val := p.curToken.Value
		p.nextToken()
		for p.curToken.Type != lexer.TokError && !p.curToken.Type.IsOneOf(endToken...) {
			val += p.curToken.Value
			p.nextToken()
		}

		p.ignoreWhitespaces()

		simpleCmd.Suffix.Words = append(simpleCmd.Suffix.Words, val)
	}

	// Handle suffixes.
	simpleCmd.Suffix.Redirects = parseCommandRedirect(p)

	p.expect(endToken...)
	return simpleCmd
}

func parseCommandPrefix(p *parser) ast.CmdPrefix {
	var assignments []string
	var redirects []ast.IORedirect
	for {
		assign := parseVariableAssignments(p)
		reds := parseCommandRedirect(p)
		if len(assign) == 0 && len(reds) == 0 {
			return ast.CmdPrefix{
				Assignments: assignments,
				Redirects:   redirects,
			}
		}
		assignments = append(assignments, assign...)
		redirects = append(redirects, reds...)
	}
}

func parseVariableAssignments(p *parser) []string {
	var out []string

	for {
		p.ignoreWhitespaces()

		if !p.curToken.Type.IsOneOf(lexer.TokIdentifier, lexer.TokSingleQuoteString, lexer.TokDoubleQuoteString, lexer.TokNumber) {
			return out
		}
		if p.peek().Type != lexer.TokEquals {
			return out
		}
		name := p.expectIdentifierStr().Value
		p.nextToken() // Consume the var name.
		p.nextToken() // Consume the equals sign.
		out = append(out, name+"="+p.expectIdentifierStr().Value)
		p.nextToken() // Consume the var value.
	}
}

func parseCommandRedirect(p *parser) []ast.IORedirect {
	var redirects []ast.IORedirect
	for {
		p.ignoreWhitespaces()
		switch p.curToken.Type {
		case lexer.TokRedirectGreatAnd, lexer.TokRedirectLessAnd:
			// Parse the fd number.
			fd, err := strconv.Atoi(p.curToken.Value)
			if err != nil {
				panic(fmt.Errorf("invalid fd number: %q", p.curToken.Value))
			}
			op := p.curToken.Type
			p.nextToken()
			p.ignoreWhitespaces()
			red := ast.IORedirect{
				Number: fd,
				Op:     op,
			}

			target := p.expectIdentifierStr().Value
			if p.curToken.Type == lexer.TokNumber {
				n, err := strconv.Atoi(target)
				if err != nil {
					panic(fmt.Errorf("invalid target fd number: %q", target))
				}
				red.ToNumber = &n
			} else {
				if op == lexer.TokRedirectLessAnd {
					panic(fmt.Errorf("file number expected after %q", op))
				}
				red.Filename = target
			}

			p.nextToken()
			redirects = append(redirects, red)

		case lexer.TokRedirectLess, lexer.TokRedirectGreat, lexer.TokRedirectDoubleGreat, lexer.TokRedirectLessGreat:
			// Parse the fd number.
			fd, err := strconv.Atoi(p.curToken.Value)
			if err != nil {
				panic(fmt.Errorf("invalid fd number: %q", p.curToken.Value))
			}
			op := p.curToken.Type
			p.nextToken()
			p.ignoreWhitespaces()
			red := ast.IORedirect{
				Number:   fd,
				Op:       op,
				Filename: p.expectIdentifierStr().Value,
			}
			p.nextToken()
			redirects = append(redirects, red)

		case lexer.TokRedirectDoubleLess:
			// Parse the fd number.
			fd, err := strconv.Atoi(p.curToken.Value)
			if err != nil {
				panic(fmt.Errorf("invalid fd number: %q", p.curToken.Value))
			}
			p.nextToken() // Consume the fd number.
			p.ignoreWhitespaces()
			hereEnd := p.expectIdentifierStr().Value
			p.nextToken() // Consume the hereEnd token.
			p.ignoreWhitespaces()
			p.expect(lexer.TokNewline) // We exepect a newline token here.
			p.nextToken()              // Consume the newline token.

			// Consume all tokens until we reach the hereEnd token.
			hereDoc := ""
			for !p.curToken.Type.IsOneOf(lexer.TokEOF, lexer.TokError) && p.curToken.Value != hereEnd {
				hereDoc += p.curToken.Value
				p.nextToken()
			}
			// Consume the hereEnd token.
			if p.curToken.Value == hereEnd {
				p.nextToken()
			}
			red := ast.IORedirect{
				Number:  fd,
				Op:      lexer.TokRedirectDoubleLess,
				HereDoc: hereDoc,
			}
			redirects = append(redirects, red)

		default:
			return redirects
		}
	}
}
