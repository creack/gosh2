package parser

import (
	"fmt"
	"strconv"

	"go.creack.net/gosh2/ast"
	"go.creack.net/gosh2/lexer"
)

func parseCompleteCommand(p *parser) ast.CompleteCommand {
	p.ignoreWhitespaces()
	endTokens := []lexer.TokenType{lexer.TokEOF, lexer.TokNewline, lexer.TokWhitespace}

	completeCmd := ast.CompleteCommand{}

	completeCmd.List = parseList(p, endTokens)
	if p.curToken.Type.IsOneOf(lexer.TokAmpersand, lexer.TokSemicolon) {
		completeCmd.Separator = p.prevToken.Type
		p.nextToken()
	}

	p.expect(endTokens...)
	return completeCmd
}

func parseList(p *parser, endTokens []lexer.TokenType) ast.List {
	p.ignoreWhitespaces()

	list := ast.List{}
	for {
		andOr := parseAndOr(p, endTokens)
		list.AndOrs = append(list.AndOrs, andOr)

		if p.curToken.Type.IsOneOf(lexer.TokSemicolon, lexer.TokAmpersand) {
			p.nextToken()
			// A list cannot end with a separator, if there is one, it must be followed by a and_or.
			// If it is not, it is the end of the list.
			if p.curToken.Type.IsOneOf(endTokens...) {
				return list
			}
			list.Separators = append(list.Separators, p.prevToken.Type)
			continue
		}
		break
	}

	p.expect(endTokens...)
	return list
}

func parseAndOr(p *parser, endTokens []lexer.TokenType) ast.AndOr {
	p.ignoreWhitespaces()
	endTokens = append(endTokens, lexer.TokAmpersand, lexer.TokSemicolon)

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
		cmd := parseCommand(p, endTokens)
		pipeline.Commands = append(pipeline.Commands, cmd)

		// If the next token is a pipe, continue parsing.
		if p.curToken.Type == lexer.TokPipe {
			p.nextToken()
			continue
		}

		// Otherwise, break the loop.
		break
	}

	p.expect(endTokens...)
	return pipeline
}

func parseCommand(p *parser, endToken []lexer.TokenType) ast.SimpleCommand {
	p.ignoreWhitespaces()
	endToken = append(endToken, lexer.TokPipe)

	simpleCmd := ast.SimpleCommand{}

	// Handle prefixes.
	// TODO: Support variables.
	simpleCmd.Prefix.Redirects = parseCommandRedirect(p)

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
			p.nextToken()
			val += p.curToken.Value
		}
		p.ignoreWhitespaces()

		simpleCmd.Suffix.Words = append(simpleCmd.Suffix.Words, val)
	}

	// Handle suffixes.
	simpleCmd.Suffix.Redirects = parseCommandRedirect(p)

	p.expect(endToken...)
	return simpleCmd
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
