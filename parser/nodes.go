package parser

import (
	"fmt"
	"strconv"

	"go.creack.net/gosh2/ast"
	"go.creack.net/gosh2/lexer"
)

func parseCompleteCommand(p *parser) ast.CompleteCommand {
	p.ignoreNewlines()

	completeCmd := ast.CompleteCommand{}

	completeCmd.List = parseList(p)
	if p.prevToken.Type == lexer.TokAmpersand || p.prevToken.Type == lexer.TokSemicolon {
		completeCmd.Separator = p.prevToken.Type
		p.nextToken()
	}

	p.expect(lexer.TokEOF, lexer.TokNewline)
	return completeCmd
}

func parseList(p *parser) ast.List {
	p.ignoreNewlines()

	list := ast.List{}
	for {
		andOr := parseAndOr(p)
		list.AndOrs = append(list.AndOrs, andOr)

		if p.curToken.Type == lexer.TokSemicolon || p.curToken.Type == lexer.TokAmpersand {
			p.nextToken()
			// A list cannot end with a separator, if there is one, it must be followed by a and_or.
			// If it is not, it is the end of the list.
			if p.curToken.Type == lexer.TokEOF || p.curToken.Type == lexer.TokNewline {
				return list
			}
			list.Separators = append(list.Separators, p.prevToken.Type)
			continue
		}
		break
	}

	p.expect(lexer.TokEOF, lexer.TokNewline)
	return list
}

func parseAndOr(p *parser) ast.AndOr {
	p.ignoreNewlines()

	andOr := ast.AndOr{}
	for {
		pipeline := parsePipeline(p)
		andOr.Pipelines = append(andOr.Pipelines, pipeline)
		if p.curToken.Type == lexer.TokLogicalAnd || p.curToken.Type == lexer.TokLogicalOr {
			andOr.Operators = append(andOr.Operators, p.curToken.Type)
			p.nextToken()
			continue
		}
		break
	}

	p.expect(lexer.TokEOF, lexer.TokNewline, lexer.TokAmpersand, lexer.TokSemicolon)
	return andOr
}

func parsePipeline(p *parser) ast.Pipeline {
	p.ignoreNewlines()

	pipeline := ast.Pipeline{}

	// Check for negation at the start of the pipeline.
	if p.curToken.Type == lexer.TokBang {
		pipeline.Negated = true
		p.nextToken()
	}

	// Parse the commands.
	for {
		cmd := parseCommand(p)
		pipeline.Commands = append(pipeline.Commands, cmd)

		// If the next token is a pipe, continue parsing.
		if p.curToken.Type == lexer.TokPipe {
			p.nextToken()
			continue
		}

		// Otherwise, break the loop.
		break
	}

	p.expect(lexer.TokEOF, lexer.TokNewline, lexer.TokAmpersand, lexer.TokSemicolon, lexer.TokLogicalAnd, lexer.TokLogicalOr)
	return pipeline
}

func parseCommand(p *parser) ast.SimpleCommand {
	p.ignoreNewlines()

	simpleCmd := ast.SimpleCommand{}

	// Handle prefixes.
	// TODO: Support variables.
	simpleCmd.Prefix.Redirects = parseCommandRedirect(p)

	simpleCmd.Name = p.expect(lexer.TokIdentifier).Value
	p.nextToken()
	for p.curToken.Type == lexer.TokIdentifier || p.curToken.Type == lexer.TokString {
		simpleCmd.Suffix.Words = append(simpleCmd.Suffix.Words, p.curToken.Value)
		p.nextToken()
	}

	// Handle suffixes.
	simpleCmd.Suffix.Redirects = parseCommandRedirect(p)

	p.expect(lexer.TokEOF, lexer.TokNewline, lexer.TokAmpersand, lexer.TokSemicolon, lexer.TokPipe, lexer.TokLogicalAnd, lexer.TokLogicalOr)
	return simpleCmd
}

func parseCommandRedirect(p *parser) []ast.IORedirect {
	var redirects []ast.IORedirect
	for {
		switch p.curToken.Type {
		case lexer.TokRedirectGreatAnd, lexer.TokRedirectLessAnd:
			// Parse the fd number.
			fd, err := strconv.Atoi(p.curToken.Value)
			if err != nil {
				panic(fmt.Errorf("invalid fd number: %q", p.curToken.Value))
			}
			op := p.curToken.Type
			p.nextToken()
			red := ast.IORedirect{
				Number: fd,
				Op:     op,
			}

			target := p.expect(lexer.TokString, lexer.TokIdentifier, lexer.TokNumber).Value
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
			red := ast.IORedirect{
				Number:   fd,
				Op:       op,
				Filename: p.expect(lexer.TokString, lexer.TokIdentifier).Value,
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
			hereEnd := p.expect(lexer.TokString, lexer.TokIdentifier).Value
			p.nextToken()              // Consume the hereEnd token.
			p.expect(lexer.TokNewline) // We exepect a newline token here.
			p.nextToken()              // Consume the newline token.

			// Consume all tokens until we reach the hereEnd token.
			hereDoc := ""
			for p.curToken.Type != lexer.TokEOF && p.curToken.Type != lexer.TokError && p.curToken.Value != hereEnd {
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
