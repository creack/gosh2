package ast

import (
	"fmt"
	"strings"

	"go.creack.net/gosh2/lexer"
)

// Structure following the posix shell grammar as defined in
//   https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_10_02

// Node is the interface that all AST nodes implement.
type Node interface {
	Dump() string
}

// Program represents the top-level program.
type Program struct {
	Commands []CompleteCommand // Represents a list of complete commands, i.e. separated by newlines.
}

func (p Program) Dump() string {
	result := "Program:\n"
	for _, cmd := range p.Commands {
		result += fmt.Sprintf("  %s\n", cmd.Dump())
	}
	return result
}

// CompleteCommand represents a complete command with optional final separator.
// CompleteCommand : list separator_op | list.
type CompleteCommand struct {
	List      List            // The list of commands, separated by ; or &.
	Separator lexer.TokenType // ";", "&", or empty.
}

func (c CompleteCommand) Dump() string {
	if c.Separator != lexer.TokEOF {
		return fmt.Sprintf("%s %s", c.List.Dump(), c.Separator)
	}
	return c.List.Dump()
}

func (c *CompleteCommand) SetList(list List) {
	c.List = list
}

func (c *CompleteCommand) SetSeparator(sep lexer.TokenType) {
	c.Separator = sep
}

// List represents a list of And/Or connected pipelines.
//
// List : list separator_op and_or | and_or.
type List struct {
	AndOrs     []AndOr           // List of AndOrs.
	Separators []lexer.TokenType // ";", "&", or empty.
}

func (l *List) AppendAndOr(andOr AndOr) {
	l.AndOrs = append(l.AndOrs, andOr)
}

func (l *List) AppendSeparator(sep lexer.TokenType) {
	l.Separators = append(l.Separators, sep)
}

func (l List) Dump() string {
	return Term(l).Dump()
}

// AndOr represents a pipeline or pipelines connected with && or ||.
type AndOr struct {
	Pipelines []Pipeline
	Operators []lexer.TokenType // "&&" or "||" between pipelines.
}

func (a AndOr) Dump() string {
	out := ""
	for i, pipeline := range a.Pipelines {
		out += pipeline.Dump()
		if i < len(a.Operators) {
			switch a.Operators[i] {
			case lexer.TokLogicalAnd:
				out += " &&"
			case lexer.TokLogicalOr:
				out += " ||"
			}
		}
	}
	return out
}

// Pipeline represents a sequence of commands connected by pipes.
type Pipeline struct {
	Commands []Command
	Negated  bool // True if pipeline starts with !.
}

func (p Pipeline) Dump() string {
	result := ""
	if p.Negated {
		result += "! "
	}
	var cmds []string
	for _, cmd := range p.Commands {
		cmds = append(cmds, cmd.Dump())
	}

	return result + strings.Join(cmds, " | ")
}

// Command represents any command (simple or compound).
type Command interface {
	Node
	CommandDump()
	GetRedirects() []IORedirect
}

// SimpleCommand represents a basic command with name, arguments and redirections.
type SimpleCommand struct {
	Prefix CmdPrefix
	Name   string // The command name.
	Suffix CmdSuffix
}

func (s SimpleCommand) GetRedirects() []IORedirect {
	return append(s.Prefix.Redirects, s.Suffix.Redirects...)
}

func (s SimpleCommand) Dump() string {
	out := s.Prefix.Dump()
	if out != "" {
		out += " "
	}
	out += s.Name
	suffix := s.Suffix.Dump()
	if suffix != "" {
		out += " " + suffix
	}
	return out
}

func (s SimpleCommand) CommandDump() {}

// CmdPrefix represents command prefixes (redirections and assignments before command).
type CmdPrefix struct {
	Redirects   []IORedirect
	Assignments []string
}

func (c CmdPrefix) Dump() string {
	var out []string
	for _, redir := range c.Redirects {
		out = append(out, redir.Dump())
	}
	out = append(out, c.Assignments...)
	return strings.Join(out, " ")
}

// CmdSuffix represents command suffixes (arguments and redirections after command).
type CmdSuffix struct {
	Words     []string
	Redirects []IORedirect
}

func (c CmdSuffix) Dump() string {
	out := c.Words
	for _, redir := range c.Redirects {
		out = append(out, redir.Dump())
	}
	return strings.Join(out, " ")
}

// CompoundCommand represents a complex command structure.
type CompoundCommand struct {
	Type         string // "brace", "subshell", "for", "case", "if", "while", "until".
	Body         Node   // Specific structure based on type.
	Redirections []IORedirect
}

func (c CompoundCommand) GetRedirects() []IORedirect {
	return c.Redirections
}

func (c CompoundCommand) Dump() string {
	switch c.Type {
	case "subshell":
		return "( " + c.Body.Dump() + " )"
	default:
		panic("Unsupported compound command type: " + c.Type)
	}
}

func (c CompoundCommand) CommandDump() {}

// SubShell represents a command group in parentheses.
type SubShell struct {
	List CompoundList
}

func (s SubShell) Dump() string {
	return fmt.Sprintf("SubShell: %s", s.List.Dump())
}

// CompoundList is the equivalent of complete_command but for subshells.
type CompoundList struct {
	Term      Term
	Separator lexer.TokenType // ";" or "&".
}

func (c CompoundList) Dump() string {
	out := c.Term.Dump()
	switch c.Separator {
	case lexer.TokSemicolon:
		out += ";"
	case lexer.TokAmpersand:
		out += "&"
	}
	return out
}

func (c *CompoundList) SetList(term Term) {
	c.Term = term
}

func (c *CompoundList) SetSeparator(sep lexer.TokenType) {
	c.Separator = sep
}

// Term is the equivalent of list but for subshells.
type Term struct {
	AndOrs     []AndOr
	Separators []lexer.TokenType // ";" or "&" between and_ors.
}

func (t Term) Dump() string {
	out := ""
	for i, andOr := range t.AndOrs {
		out += andOr.Dump()
		if i < len(t.Separators) {
			switch t.Separators[i] {
			case lexer.TokSemicolon:
				out += ";"
			case lexer.TokAmpersand:
				out += "&"
			}
		}
	}
	return out
}

func (t *Term) AppendAndOr(andOr AndOr) {
	t.AndOrs = append(t.AndOrs, andOr)
}

func (t *Term) AppendSeparator(sep lexer.TokenType) {
	t.Separators = append(t.Separators, sep)
}

// IORedirect represents an I/O redirection.
type IORedirect struct {
	Number   int             // Source file descriptor number.
	Op       lexer.TokenType // "<", "<&", ">", ">&", ">>", "<>", ">|".
	ToNumber *int            // For n>&m, nil if not specified.
	Filename string          // For file redirections.
	HereDoc  string          // For heredocs.
}

func (i IORedirect) Dump() string {
	out := fmt.Sprintf("%d%s", i.Number, i.Op)
	if i.ToNumber != nil {
		out += fmt.Sprintf("%d", *i.ToNumber)
	}
	if i.Filename != "" {
		out += i.Filename
	}
	if i.HereDoc != "" {
		out += i.HereDoc
	}
	return out
}
