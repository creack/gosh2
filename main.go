package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/kr/pretty"
	"go.creack.net/gosh2/ast"
	"go.creack.net/gosh2/parser"
)

func test() (int, error) {
	input := ""
	input = "echo hello > foo; echo world >> foo; ls /dev/fd 7<foo; cat /dev/fd/7 7<foo"
	input = "foo.sh 8> ret; echo why && echo ok1 || echo ko2 && echo ok2; cat ret; echo -1-"
	input = "echo hello > foo; foo.sh <> foo; echo --; cat foo"
	// input = "foo.sh 7<foo | cat -e; echo --; ls; echo --; cat a"
	// input = "cat /dev/fd/9 9<&7 7<foo"
	// input = "echo hello 8>bar >&8; cat bar"
	input = "echo ___; cat -e<<EOF\nhello\nworld\nEOF\necho ^^^^"
	input = `myecho "\a\b\\\a" '\a\b\\\a' \a\b\\\a`
	input = `echo hello\"world`
	input = "echo --; cat foo"
	input = "rm -f foo bar; ls -l > bar | foo.sh | wc -c | cat -e > foo; echo --; cat bar; echo --; cat foo"
	input = `echo 'hello\
world'''a`
	input = `echo [?b]`
	input = "echo hello > foo; foo.sh <> foo; echo --; cat foo"
	input = `fooa=bar >bar foo=foo sh -c 'echo $foo'; cat -e bar`
	input = "/bin/echo `/bin/echo \\`/bin/echo hello\\``"
	input = "echo z$(echo b$(echo c$(echo d$(echo ehello))))a"
	input = "echo a`ls`b"
	input = "echo a`exit 1`;echo bb"
	input = "echo a`sh -c \"echo oka; echo okb >&2; echo okc\"`b"
	input = "(echo a`sh -c \"echo oka; echo okb >&2; echo okc\"`b 2>&1) | cat -e"
	input = "(echo hello > foo1; cat foo1)"
	input = "echo hellor 9> foo >&9;cat foo"
	input = "foooobarrrr=bar1 env > foo; cat foo"
	input = "cd /tmp; pwd; (cd /Volumes; pwd); pwd"
	//input = "echo hello; cat foo"
	//input = "echo hello && cat foo"
	input = "cat<<EOF\nEOF\n"
	input = "<foo cat"

	// TODO: Handle this case
	//input = "mkdir -p aaaa1234; cd aaaa1234; echo hello1 8>bar33 >&8; ls; cat bar33"

	input = "rm -f bar33; echo hello1 8<>bar33 >&8; ls; cat bar33"
	input = "cat -e<<EOF\nEOF"
	input = "echo `echo hello`"
	input = "(echo hello)>foo; cat foo"
	input = "echo hello>foo; cat foo"
	input = "echo hello | cat -e | cat -e"
	input = "a=b echo a b c > f 8>&1 >> foo < bar <<-HERE"
	input = "a=b echo"
	input = "ls a aa > foo; cat foo"
	input = "< foo wc | cat -e"
	//input = "cat foo | wc | cat -e"
	input = "rm -f bar; < foo cat | sh -c 'cat 0<&6' 6<foo | wc | cat -e > bar; echo ---; cat bar"
	input = "(echo hello) | cat -e"
	input = "cat -e <<EOF\nhello\nworld\nEOF\necho a"
	input = "(echo hello >&8) 8> ret; cat -e ret"
	input = "myecho a\"b\"'c'a"
	input = "echo a`ls b`c"
	input = "(echo a>&2; echo b) | cat -e"
	input = "foo.sh 2>&1 | cat -e"
	input = "echo hello0 >&2"
	input = "echo hello2 | wc -l | cat -e"
	input = "echo hello1 >&2 | cat -e"
	input = "foo.sh 1>&2 | cat -e"
	input = "echo oka; echo okb >&2; echo okc"
	input = "(echo `echo 'oka'; echo \"okb\" >&2; echo okc`) > /dev/null 2> foo; cat -e foo"

	if true {
		cmd := exec.Command("bash", "--posix")
		cmd.Stdin = strings.NewReader(input)
		outBuf := bytes.NewBuffer(nil)
		errBuf := bytes.NewBuffer(nil)
		cmd.Stdout = outBuf
		cmd.Stderr = errBuf
		fmt.Printf("------POSIX-----\n")
		if err := cmd.Run(); err != nil {
			log.Printf("sh error: %s.", err)
		}
		fmt.Printf("[POSIXOUT] %q\n", outBuf.String())
		fmt.Printf("[POSIXERR] %q\n", errBuf.String())
		fmt.Printf("------!POSIX-----\n")
		fmt.Printf("------GOSH-------\n")
		defer fmt.Printf("------!gosh-----\n")
	}
	if false {
		p := parser.New(strings.NewReader(input))
		for {
			cmd := p.NextCompleteCommand()
			if cmd == nil {
				break
			}
			fmt.Printf("------>>\n%s\n^^^^\n", cmd.Dump())

			break
			pretty.Println(cmd.List)
		}
	}

	if false {
		p := parser.New(strings.NewReader(input))
		completeCommand := p.NextCompleteCommand()
		list := completeCommand.List
		fmt.Println(list.Left.Left.Right.Right.Right.Right.(*ast.SimpleCommand).Suffix.Words)
		fmt.Println(list.Left.Separator)
		fmt.Println(list.Left.Right.Right.Right.Right.(*ast.SimpleCommand).Suffix.Words)
		fmt.Println(list.Separator)
		fmt.Println(list.Right.Right.Right.Right.(*ast.SimpleCommand).Suffix.Words)
	}

	outBuf := bytes.NewBuffer(nil)
	errBuf := bytes.NewBuffer(nil)
	exitCode, err := parser.Run(strings.NewReader(input), os.Stdin, outBuf, errBuf)
	fmt.Printf("[OUT] %q\n", outBuf.String())
	fmt.Printf("[ERR] %q\n", errBuf.String())
	return exitCode, err
}

func main() {
	if parser.RunSubshell(os.Args, os.Exit, os.Stdin, os.Stdout, os.Stderr) {
		return
	}
	for i := 3; i <= 7; i++ {
		_ = os.NewFile(uintptr(i), "").Close()
	}
	wd, _ := os.Getwd()
	os.Setenv("PATH", os.Getenv("PATH")+":"+wd)
	tmpDir, _ := os.MkdirTemp("/tmp", "gosh2")
	defer func() { _ = os.RemoveAll(tmpDir) }() // Best effort cleanup.
	if err := os.Chdir(tmpDir); err != nil {
		log.Fatalf("Fail: %s.", err)
	}
	if err := os.WriteFile("foo", []byte("foocontent\n"), 0644); err != nil {
		log.Fatalf("Fail: %s.", err)
	}
	exec.Command("touch", "b", "bb", "a", "aa", "ast", "bara", "foo", "foo.sh", "go.mod", "go.sum", "lexer", "tmp", "sh").Run()

	lastExitCode, err := test()
	_ = os.RemoveAll(tmpDir) // Best effort cleanup.
	if err != nil {
		log.Fatalf("Fail: %s.", err)
	}
	os.Exit(lastExitCode)
}
