package ast

import (
	"fmt"

	"go.creack.net/gosh2/lexer"
)

// Structure following the posix shell grammar as defined in
//   https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_10_02

// Program represents the top-level program.
type Program struct {
	Commands []CompleteCommand // Represents a list of complete commands, i.e. separated by newlines.
}

func (p Program) Dump() string {
	result := ""
	for _, cmd := range p.Commands {
		result += fmt.Sprintf("%s\n", cmd.Dump())
	}
	return result
}

// CompleteCommand : list separator_op | list.
type CompleteCommand struct {
	List      *List           // The list of commands, separated by ; or &.
	Separator lexer.TokenType // ";", "&", or empty.
}

func (c CompleteCommand) Dump() string {
	str := c.List.Dump()
	if c.Separator != lexer.TokEOF && c.Separator != lexer.TokError {
		str += c.Separator.String()
	}
	return str
}

// List : list separator_op and_or | and_or.
type List struct {
	Left      *List
	Separator lexer.TokenType // ";", "&", or empty.
	Right     *AndOr
}

func (l List) Dump() string {
	if l.Left == nil {
		return l.Right.Dump()
	}
	return fmt.Sprintf("%s%s %s", l.Left.Dump(), l.Separator, l.Right.Dump())
}

// AndOr represents a pipeline or pipelines connected with && or ||.
type AndOr struct {
	Left      *AndOr
	Separator lexer.TokenType // "&&" or "||".
	Right     *Pipeline
}

func (a AndOr) Dump() string {
	if a.Left == nil {
		return a.Right.Dump()
	}
	return fmt.Sprintf("%s %s %s", a.Left.Dump(), a.Separator, a.Right.Dump())
}

type Pipeline struct {
	Negated bool // True if pipeline starts with !.
	Right   *PipelineSequence
}

func (p Pipeline) Dump() string {
	result := ""
	if p.Negated {
		result += "! "
	}
	if p.Right == nil {
		return result
	}
	return result + p.Right.Dump()
}

// PipelineSequence represents a sequence of commands connected by pipes.
type PipelineSequence struct {
	Left  *PipelineSequence
	Right Command
}

func (p PipelineSequence) Dump() string {
	if p.Left == nil {
		return p.Right.Dump()
	}
	return fmt.Sprintf("%s | %s", p.Left.Dump(), p.Right.Dump())
}

// Command represents any command (simple or compound).
type Command interface {
	Dump() string
	command()
	IORedirects() []IORedirect
}

type CompoundCommand interface {
	Dump() string
	compoundCommand()
}

type CompoundCommandWrap struct {
	CompoundCommand
	Redir []IORedirect
}

func (CompoundCommandWrap) command()                     {}
func (cc CompoundCommandWrap) IORedirects() []IORedirect { return cc.Redir }
func (cc CompoundCommandWrap) Dump() string {
	out := cc.CompoundCommand.Dump()
	for _, r := range cc.Redir {
		out += " " + r.Dump()
	}
	return out
}

type SubshellCommand struct {
	Right *CompoundList
}

func (SubshellCommand) compoundCommand() {}

func (s SubshellCommand) Dump() string {
	if s.Right == nil {
		return "()"
	}
	return fmt.Sprintf("(%s)", s.Right.Dump())
}

type CompoundList struct {
	Term      *Term
	Separator lexer.TokenType // separator.
}

func (c CompoundList) Dump() string {
	out := c.Term.Dump()
	if c.Separator != 0 {
		out += c.Separator.String()
	}
	return out
}

type Term struct {
	Left      *Term
	Separator lexer.TokenType // separator.
	Right     *AndOr
}

func (t Term) Dump() string {
	if t.Left == nil {
		return t.Right.Dump()
	}
	return fmt.Sprintf("%s %s %s", t.Left.Dump(), t.Separator, t.Right.Dump())
}

// SimpleCommand represents a basic command with name, arguments and redirections.
type SimpleCommand struct {
	Prefix *CmdPrefix
	Name   string // cmd_word or cmd_name.
	Suffix *CmdSuffix
}

func (s SimpleCommand) IORedirects() []IORedirect {
	if s.Prefix == nil && s.Suffix == nil {
		return nil
	}
	if s.Prefix == nil {
		return s.Suffix.IORedirects()
	}
	if s.Suffix == nil {
		return s.Prefix.IORedirects()
	}
	return append(s.Prefix.IORedirects(), s.Suffix.IORedirects()...)
}

func (s SimpleCommand) Dump() string {
	out := ""
	if s.Prefix != nil {
		out += s.Prefix.Dump()
	}
	if out != "" {
		out += " "
	}
	out += s.Name
	if s.Suffix != nil {
		out += " " + s.Suffix.Dump()
	}
	return out
}

func (s SimpleCommand) command() {}

type CmdPrefix struct {
	Left           *CmdPrefix
	AssignmentWord string
	Redir          *IORedirect
}

func (c CmdPrefix) AssignmentWords() []string {
	if c.Left == nil && c.AssignmentWord == "" {
		return nil
	}
	if c.Left == nil {
		return []string{c.AssignmentWord}
	}
	if c.AssignmentWord == "" {
		return c.Left.AssignmentWords()
	}
	return append(c.Left.AssignmentWords(), c.AssignmentWord)
}

func (c CmdPrefix) IORedirects() []IORedirect {
	if c.Left == nil && c.Redir == nil {
		return nil
	}
	if c.Left == nil {
		return []IORedirect{*c.Redir}
	}
	if c.Redir == nil {
		return c.Left.IORedirects()
	}
	return append(c.Left.IORedirects(), *c.Redir)
}

func (c CmdPrefix) Dump() string {
	if c.Left == nil {
		if c.AssignmentWord != "" {
			return c.AssignmentWord
		}
		return c.Redir.Dump()
	}
	str := c.AssignmentWord
	if str == "" {
		str = c.Redir.Dump()
	}
	return fmt.Sprintf("%s %s", c.Left.Dump(), str)
}

type CmdSuffix struct {
	Left  *CmdSuffix
	Word  string
	Redir *IORedirect
}

func (c CmdSuffix) Words() []string {
	if c.Left == nil && c.Word == "" {
		return nil
	}
	if c.Left == nil {
		return []string{c.Word}
	}
	if c.Word == "" {
		return c.Left.Words()
	}
	return append(c.Left.Words(), c.Word)
}

func (c CmdSuffix) IORedirects() []IORedirect {
	if c.Left == nil && c.Redir == nil {
		return nil
	}
	if c.Left == nil {
		return []IORedirect{*c.Redir}
	}
	if c.Redir == nil {
		return c.Left.IORedirects()
	}
	return append(c.Left.IORedirects(), *c.Redir)
}

func (c CmdSuffix) Dump() string {
	if c.Left == nil {
		if c.Word != "" {
			return c.Word
		}
		return c.Redir.Dump()
	}
	str := c.Word
	if str == "" {
		str = c.Redir.Dump()
	}
	return fmt.Sprintf("%s %s", c.Left.Dump(), str)
}

type IORedirect struct {
	Number int
	IOFile IOFile
}

func (i IORedirect) Dump() string {
	return fmt.Sprintf("%d%s", i.Number, i.IOFile.Dump())
}

// IOFile represents io_file and io_here.
type IOFile struct {
	Operator lexer.TokenType // "<", ">", ">>", "|&", etc.
	Filename string          // Filename or hereend.
	ToNumber *int            // For n>&m, nil if not specified.
}

func (i IOFile) Dump() string {
	if i.ToNumber != nil {
		return fmt.Sprintf("%s%d", i.Operator, *i.ToNumber)
	}
	return fmt.Sprintf("%s%s", i.Operator, i.Filename)
}
