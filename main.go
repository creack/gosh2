package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/kr/pretty"
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

	cmd := exec.Command("bash", "--posix")
	cmd.Stdin = strings.NewReader(input)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Printf("------POSIX-----\n")
	if err := cmd.Run(); err != nil {
		log.Printf("sh error: %s.", err)
	}
	fmt.Printf("------!POSIX-----\n")
	fmt.Printf("------GOSH-------\n")
	defer fmt.Printf("------!gosh-----\n")

	if true {
		p := parser.New(strings.NewReader(input))
		for {
			cmd := p.NextCompleteCommand()
			if cmd == nil {
				break
			}
			pretty.Println(cmd)
		}
	}
	return parser.Run(strings.NewReader(input), os.Stdin, os.Stdout, os.Stderr)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-sub" {
		exitCode, err := parser.Run(os.Stdin, nil, os.Stdout, os.Stderr)
		if exitCode == 0 && err != nil {
			log.Fatalf("Sub fail: %s.", err)
		}
		os.Exit(exitCode)
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
	if err := os.WriteFile("foo", []byte("foo\n"), 0644); err != nil {
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
