package ast

import (
	"fmt"

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

// List represents a list of And/Or connected pipelines.
//
// List : list separator_op and_or | and_or.
type List struct {
	AndOrs     []AndOr           // List of AndOrs.
	Separators []lexer.TokenType // ";", "&", or empty.
}

func (l List) Dump() string {
	result := "List:"
	for _, ao := range l.AndOrs {
		result += " " + ao.Dump()
	}
	return result
}

// AndOr represents a pipeline or pipelines connected with && or ||.
type AndOr struct {
	Pipelines []Pipeline
	Operators []lexer.TokenType // "&&" or "||" between pipelines.
}

func (a AndOr) Dump() string {
	if len(a.Pipelines) == 1 {
		return a.Pipelines[0].Dump()
	}

	result := "AndOr:"
	for i, p := range a.Pipelines {
		result += " " + p.Dump()
		if i < len(a.Operators) {
			result += " " + a.Operators[i].String()
		}
	}
	return result
}

// Pipeline represents a sequence of commands connected by pipes.
type Pipeline struct {
	Commands []Command
	Negated  bool // True if pipeline starts with !.
}

func (p Pipeline) Dump() string {
	result := "Pipeline:"
	if p.Negated {
		result += " !"
	}
	for _, cmd := range p.Commands {
		result += " " + cmd.Dump()
	}
	return result
}

// Command represents any command (simple or compound).
type Command interface {
	Node
	CommandDump()
}

// SimpleCommand represents a basic command with name, arguments and redirections.
type SimpleCommand struct {
	Prefix CmdPrefix
	Name   string // The command name.
	Suffix CmdSuffix
}

func (s SimpleCommand) Dump() string {
	return fmt.Sprintf("SimpleCommand: %s", s.Name)
}

func (s SimpleCommand) CommandDump() {}

// CmdPrefix represents command prefixes (redirections and assignments before command).
type CmdPrefix struct {
	Redirects   []IORedirect
	Assignments []string
}

// CmdSuffix represents command suffixes (arguments and redirections after command).
type CmdSuffix struct {
	Words     []string
	Redirects []IORedirect
}

// CompoundCommand represents a complex command structure.
type CompoundCommand struct {
	Type         string // "brace", "subshell", "for", "case", "if", "while", "until".
	Body         Node   // Specific structure based on type.
	Redirections []IORedirect
}

func (c CompoundCommand) Dump() string {
	return fmt.Sprintf("CompoundCommand(%s): %v", c.Type, c.Body)
}

func (c CompoundCommand) CommandDump() {}

// SubShell represents a command group in parentheses.
type SubShell struct {
	List CompoundList
}

func (s SubShell) Dump() string {
	return fmt.Sprintf("SubShell: %s", s.List.Dump())
}

// CompoundList represents a list of commands that form a block.
type CompoundList struct {
	Terms []Term
}

func (c CompoundList) Dump() string {
	result := "CompoundList:"
	for _, t := range c.Terms {
		result += " " + t.Dump()
	}
	return result
}

// Term represents commands separated by ; or &.
type Term struct {
	AndOrs     []AndOr
	Separators []lexer.TokenType // ";" or "&" between and_ors.
}

func (t Term) Dump() string {
	return fmt.Sprintf("Term: %v", t.AndOrs)
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
	// TODO: Add support for ToNumber.
	return fmt.Sprintf("%d%s %s", i.Number, i.Op, i.Filename)
}
