package parser

import (
	"fmt"
	"strconv"

	"go.creack.net/gosh2/ast"
	"go.creack.net/gosh2/lexer"
)

func parseCompleteCommand(p *parser) *ast.CompleteCommand {
	p.ignoreNLWhitespaces()
	if p.curToken.Type == lexer.TokEOF {
		return nil
	}

	ccmd := &ast.CompleteCommand{
		List: parseList(p, 0, nil),
	}
	if ccmd.List.Right == nil {
		ccmd.Separator = ccmd.List.Separator
		ccmd.List = ccmd.List.Left
	}

	p.expect(lexer.TokEOF, lexer.TokNewline)
	return ccmd
}

func parseList(p *parser, sep lexer.TokenType, parent *ast.List) *ast.List {
	p.ignoreWhitespaces()

	list := &ast.List{
		Left:      parent,
		Separator: sep,
		Right:     parseAndOr(p, 0, nil),
	}

	// If we don't have a separator_op, there is no left side and we are done.
	if !p.curToken.Type.IsOneOf(lexer.TokSeparatorOp...) {
		return list
	}

	// Otherwise, we have a separator_op and we parsed the need to parse left side.
	nextSep := p.curToken.Type
	p.nextToken() // Consume the separator_op.
	return parseList(p, nextSep, list)
}

func parseAndOr(p *parser, sep lexer.TokenType, parent *ast.AndOr) *ast.AndOr {
	p.ignoreWhitespaces()

	if p.curToken.Type == lexer.TokEOF {
		return nil
	}

	andOr := &ast.AndOr{
		Left:      parent,
		Separator: sep,
		Right:     parsePipeline(p),
	}

	// If we don't have a AND_IF or OR_IF, pipeline is the right side and we are done.
	if !p.curToken.Type.IsOneOf(lexer.TokAndIf, lexer.TokOrIf) {
		return andOr
	}

	// Otherwise, we have a AND_IF or OR_IF and we need to parse the left side.
	nextSep := p.curToken.Type
	p.nextToken() // Consume the AND_IF or OR_IF.
	return parseAndOr(p, nextSep, andOr)
}

func parsePipeline(p *parser) *ast.Pipeline {
	p.ignoreWhitespaces()

	pipeline := &ast.Pipeline{}

	// Check for negation at the start of the pipeline.
	if p.curToken.Type == lexer.TokBang {
		pipeline.Negated = true
		p.nextToken()
		p.expect(lexer.TokWhitespace)
		p.ignoreWhitespaces()
	}

	pipeline.Right = parsePipelineSequence(p, nil)

	return pipeline
}

func parsePipelineSequence(p *parser, parent *ast.PipelineSequence) *ast.PipelineSequence {
	pipelineSeq := &ast.PipelineSequence{
		Left:  parent,
		Right: parseCommand(p),
	}
	p.ignoreWhitespaces()

	// If we don't have a pipe, there is no left side and we are done.
	if p.curToken.Type != lexer.TokPipe {
		return pipelineSeq
	}
	p.nextToken() // Consume the pipe.

	// Otherwise, we have a pipe and we need to parse the left side.
	return parsePipelineSequence(p, pipelineSeq)

}

func parseCommand(p *parser) ast.Command {
	p.ignoreWhitespaces()

	switch p.curToken.Type {
	case lexer.TokParenLeft:
		return parseCompoundCommand(p)
	default:
		return parseSimpleCommand(p)
	}
}

func parseCompoundCommand(p *parser) *ast.CompoundCommandWrap {
	compoundCmd := &ast.CompoundCommandWrap{}

	switch p.curToken.Type {
	case lexer.TokParenLeft:
		compoundCmd.CompoundCommand = parseSubshell(p)
	default:
		// Cannot happen.
		panic(fmt.Errorf("unexpected token %q", p.curToken))
	}

	p.ignoreWhitespaces()
	for p.curToken.Type.IsOneOf(lexer.TokAnyRedirect...) {
		red := parseIORedirect(p)
		compoundCmd.Redir = append(compoundCmd.Redir, *red)
		//p.ignoreWhitespaces()
	}

	return compoundCmd
}

func parseSubshell(p *parser) *ast.SubshellCommand {
	p.nextToken() // Consume the left parenthesis.
	p.ignoreWhitespaces()

	subshell := &ast.SubshellCommand{
		Right: parseCompoundList(p),
	}

	p.expect(lexer.TokParenRight)
	p.nextToken() // Consume the right parenthesis.

	return subshell
}

func parseCompoundList(p *parser) *ast.CompoundList {
	p.ignoreWhitespaces()

	cList := &ast.CompoundList{
		Term: parseTerm(p, 0, nil),
	}
	if cList.Term.Right == nil {
		cList.Separator = cList.Term.Separator
		cList.Term = cList.Term.Left
	}

	return cList
}

func parseTerm(p *parser, sep lexer.TokenType, parent *ast.Term) *ast.Term {
	p.ignoreWhitespaces()

	term := &ast.Term{
		Left:      parent,
		Separator: sep,
		Right:     parseAndOr(p, 0, nil),
	}

	if !p.curToken.Type.IsOneOf(lexer.TokSeparator...) {
		return term
	}

	nextSep := p.curToken.Type
	p.nextToken() // Consume the separator.
	return parseTerm(p, nextSep, term)
}

func parseSimpleCommand(p *parser) *ast.SimpleCommand {
	p.ignoreWhitespaces()

	simpleCmd := &ast.SimpleCommand{}

	// Handle prefixes.
	simpleCmd.Prefix = parseCmdPrefix(p, nil)

	// Handle the command name.
	// TODO: Add support for `e"c"h'o' hello world`.
	simpleCmd.Name = p.expectIdentifierStr().Value
	p.nextToken()
	p.ignoreWhitespaces()

	// Handle suffixes.
	simpleCmd.Suffix = parseCmdSuffix(p, nil)

	return simpleCmd
}

func parseCmdPrefix(p *parser, parent *ast.CmdPrefix) *ast.CmdPrefix {
	p.ignoreWhitespaces()

	if p.peek().Type == lexer.TokEquals {
		prefix := &ast.CmdPrefix{
			Left:           parent,
			AssignmentWord: p.expectIdentifierStr().Value,
		}
		p.nextToken() // Consume the variable name.
		p.nextToken() // Consume the equals sign.
		prefix.AssignmentWord += "=" + p.expectIdentifierStr().Value
		p.nextToken() // Consume the variable value.
		return parseCmdPrefix(p, prefix)
	}
	if p.curToken.Type.IsOneOf(lexer.TokAnyRedirect...) {
		prefix := &ast.CmdPrefix{
			Left: parent,
		}
		prefix.Redir = parseIORedirect(p)
		return parseCmdPrefix(p, prefix)
	}

	return parent
}

func parseCmdSuffix(p *parser, parent *ast.CmdSuffix) *ast.CmdSuffix {
	if p.curToken.Type == lexer.TokNewline {
		return parent
	}
	p.ignoreWhitespaces()

	if p.curToken.Type.IsOneOf(lexer.TokIdentifier, lexer.TokSingleQuoteString, lexer.TokDoubleQuoteString, lexer.TokNumber) {
		suffix := &ast.CmdSuffix{
			Left: parent,
			Word: p.expectIdentifierStr().Value,
		}
		p.nextToken()
		return parseCmdSuffix(p, suffix)
	}
	if p.curToken.Type.IsOneOf(lexer.TokAnyRedirect...) {
		suffix := &ast.CmdSuffix{
			Left: parent,
		}
		suffix.Redir = parseIORedirect(p)
		return parseCmdSuffix(p, suffix)
	}

	return parent
}

func parseIORedirect(p *parser) *ast.IORedirect {
	p.ignoreWhitespaces()

	// Parse the fd number.
	fd, err := strconv.Atoi(p.curToken.Value)
	if err != nil {
		panic(fmt.Errorf("invalid fd number: %q", p.curToken.Value))
	}
	op := p.curToken.Type
	p.nextToken()
	p.ignoreWhitespaces()

	switch op {
	case lexer.TokRedirectGreatAnd, lexer.TokRedirectLessAnd:
		red := &ast.IORedirect{
			Number: fd,
			IOFile: ast.IOFile{
				Operator: op,
			},
		}

		target := p.expectIdentifierStr().Value
		if p.curToken.Type == lexer.TokNumber {
			n, err := strconv.Atoi(target)
			if err != nil {
				panic(fmt.Errorf("invalid target fd number: %q", target))
			}
			red.IOFile.ToNumber = &n
		} else {
			if op == lexer.TokRedirectLessAnd {
				panic(fmt.Errorf("file number expected after %q", op))
			}
			red.IOFile.Filename = target
		}
		p.nextToken() // Consume the target token.
		return red

	case lexer.TokRedirectLess, lexer.TokRedirectGreat, lexer.TokRedirectDoubleGreat, lexer.TokRedirectLessGreat:
		red := &ast.IORedirect{
			Number: fd,
			IOFile: ast.IOFile{
				Operator: op,
				Filename: p.expectIdentifierStr().Value,
			},
		}
		p.nextToken() // Consume the target token.
		return red

	case lexer.TokRedirectDoubleLess, lexer.TokRedirectDoubleLessDash:
		hereEnd := p.expectIdentifierStr().Value
		p.nextToken()                            // Consume the hereEnd token.
		p.expect(lexer.TokNewline, lexer.TokEOF) // We exepect a newline token here.
		p.nextToken()                            // Consume the newline token.

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
		red := &ast.IORedirect{
			Number: fd,
			IOFile: ast.IOFile{
				Operator: lexer.TokRedirectDoubleLess,
				Filename: hereDoc,
			},
		}
		return red

	default:
		// Should never happen.
		panic(fmt.Errorf("unsupported redirect %q", op))
	}
}
