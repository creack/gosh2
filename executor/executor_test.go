package executor_test

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.creack.net/gosh2/parser"
)

func setupEnv(t *testing.T) {
	tmpDir, err := os.MkdirTemp("/tmp", "gosh2-executor")
	require.NoError(t, err, "failed to create temp dir")
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(tmpDir)) })

	require.NoError(t, os.Chdir(tmpDir), "failed to change dir to temp dir %q", tmpDir)
	require.NoError(t, os.WriteFile("foo", []byte("foocontent\n"), 0644), "failed to write file %q", "foo")

	for _, name := range []string{
		"b", "bb", "a", "aa", "ast", "bara", "foo.sh", "go.mod", "go.sum", "lexer", "tmp", "sh",
	} {
		f, err := os.Create(name)
		require.NoError(t, err, "failed to create file %q", name)
		require.NoError(t, f.Close(), "failed to close file %q", name)
	}

	// Create bin directory and populate with fake executables (a la busybox).
	require.NoError(t, os.MkdirAll("bin", 0755), "failed to create bin dir")
	for _, name := range []string{
		"myecho",
		// TODO: Implement rm, ls, cat, cat -e.
	} {
		f, err := os.Create("bin/" + name)
		require.NoError(t, err, "failed to create file %q", name)
		assert.NoError(t, f.Chmod(0755), "failed to chmod file %q", name)
		require.NoError(t, f.Close(), "failed to close file %q", name)
	}

	require.NoError(t, os.Setenv("GOSH2_TEST", "1"), "failed to set env GOSH2_TEST")
	// TODO: Remove the parent's PATH once all binaries are implemented.
	require.NoError(t, os.Setenv("PATH", os.Getenv("PATH")+":"+tmpDir+"/bin"), "failed to set env PATH")
}

var flSub = flag.Bool("sub", false, "Run as subshell")

func TestMain(m *testing.M) {
	switch os.Args[0] {
	case "myecho":
		fmt.Printf("Args: %d\n", len(os.Args))
		fmt.Printf("%s\n", strings.Join(os.Args[1:], " "))
		fmt.Printf("%q\n", strings.Join(os.Args[1:], " "))
		os.Exit(0)
		return
	}
	flag.Parse()
	if flSub != nil && *flSub {
		exitCode, err := parser.Run(os.Stdin, nil, os.Stdout, os.Stderr)
		if exitCode == 0 && err != nil {
			log.Fatalf("Sub fail: %s.", err)
		}
		os.Exit(exitCode)
		return
	}

	os.Exit(m.Run())
}

type testCase struct {
	name     string
	input    string
	stdout   string
	stderr   string
	wantErr  bool
	exitCode int
}

func TestExecutor(t *testing.T) {
	// Run the tests.
	tests := []testCase{
		{name: "empty", input: "", stdout: ""},
		{name: "simple cmd", input: "ls a", stdout: "a\n"},
		{name: "simple cmd error", input: "ls /foo/bar/not/exists", exitCode: 1},
		{name: "simple builtin cmd", input: "echo hello", stdout: "hello\n"},
		{name: "cmd right redir", input: "ls a aa > foo; cat foo", stdout: "a\naa\n"},
		{name: "builtin double right redirect", input: "rm foo; echo hello >> foo; echo world >> foo; cat foo", stdout: "hello\nworld\n"},
		{name: "left redirect", input: "cat < foo", stdout: "foocontent\n"},
		// TODO: Fix this.
		// {name: "fd right redirect", input: "(echo hello >&8) 8> ret; cat ret", stdout: "hello\n"},
		{name: "fd right redirect", input: "echo hello 8>bar >&8; cat bar", stdout: "hello\n"},
		{name: "andors success", input: "ls a && echo why && echo ok1 || echo ko2 && echo ok2; cat foo; echo -1-", stdout: "a\nwhy\nok1\nok2\nfoocontent\n-1-\n"},
		{name: "andors failure", input: "ls /foo/bar/not/exists && echo why && echo ok1 || echo ko2 && echo ok2; cat foo; echo -1-", stdout: "ko2\nok2\nfoocontent\n-1-\n", exitCode: 0},
		// TODO: Add full set of tests for and/or, semicolumn, pipes asserting final exitcode.
		{name: "simple pipe", input: "ls a aa | cat -e", stdout: "a$\naa$\n"},
		{name: "multi pipe redirect", input: "< foo cat | cat -e | cat -e > bar; cat bar", stdout: "foocontent$$\n"},
		// TODO: Remove sh -c once we have implemented the builtin read.
		{name: "leftright redirect", input: "echo aa > foo; echo bb >> foo; sh -c 'read line; echo $line; echo cc >&0' <>foo; echo --; cat foo", stdout: "aa\n--\naa\ncc\n"},
		{name: "fd left redirect", input: "cat " + selfFD() + "/9 7<foo 9<&7", stdout: "foocontent\n"},
		// TODO: Handle this case.
		// Add simple errors like bin not found in PATH.
		// {name: "internal error", input: "cat /dev/fd/9 9<&7 7<foo", stderr: "bad file descriptor 7\n", exitCode: -1},
		// {name: "subshell internal error redirect", input: "(cat /dev/fd/9 9<&7 7<foo) >& bar; cat bar", stdout: "bad file descriptor 7\n"},

		{name: "heredoc left space", input: "echo ___; cat -e <<EOF\nhello\nworld\nEOF\necho ^^^^", stdout: "___\nhello$\nworld$\n^^^^\n"},
		{name: "heredoc space", input: "echo ___; cat -e << EOF\nhello\nworld\nEOF\necho ^^^^", stdout: "___\nhello$\nworld$\n^^^^\n"},
		{name: "heredoc no space", input: "echo ___; cat -e<<EOF\nhello\nworld\nEOF\necho ^^^^", stdout: "___\nhello$\nworld$\n^^^^\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, run(tt))
	}
}

// TODO: Replace by read -u once implemented.
func selfFD() string {
	if runtime.GOOS == "darwin" {
		return "/dev/fd"
	}
	return "/proc/self/fd"
}

func TestLeftRedirect(t *testing.T) {
	// Run the tests.
	tests := []testCase{
		{name: "suffix no space", input: "cat<foo", stdout: "foocontent\n"},
		{name: "suffix left space", input: "cat <foo", stdout: "foocontent\n"},
		{name: "suffix right space", input: "cat< foo", stdout: "foocontent\n"},
		{name: "suffix space", input: "cat < foo", stdout: "foocontent\n"},
		{name: "suffix default fd no space", input: "cat 0<foo", stdout: "foocontent\n"},
		{name: "suffix default fd space", input: "cat 0< foo", stdout: "foocontent\n"},
		{name: "suffix fd 9 no space", input: "cat " + selfFD() + "/9 9<foo", stdout: "foocontent\n"},
		{name: "suffix fd 9 space", input: "cat " + selfFD() + "/9 9< foo", stdout: "foocontent\n"},

		{name: "suffix single quote no space", input: "cat<'foo'", stdout: "foocontent\n"},
		{name: "suffix single quote space", input: "cat < 'foo'", stdout: "foocontent\n"},
		{name: "suffix double quote no space", input: "cat<\"foo\"", stdout: "foocontent\n"},
		{name: "suffix double quote space", input: "cat < \"foo\"", stdout: "foocontent\n"},
		// TODO: Handle this.
		// {name: "suffix mixed quote", input: "cat<\"f\"o'o'", stdout: "foocontent\n"},
		// {name: "suffix subst no space", input: "<$(echo foo) cat", stdout: "foocontent\n"},
		// {name: "suffix subst space", input: " < $(echo foo) cat", stdout: "foocontent\n"},
		// {name: "suffix backtick no space", input: "<`echo foo` cat", stdout: "foocontent\n"},
		// {name: "suffix backtick space", input: " < `echo foo` cat", stdout: "foocontent\n"},

		{name: "prefix no space", input: "<foo cat", stdout: "foocontent\n"},
		{name: "prefix right space", input: "< foo cat", stdout: "foocontent\n"},
		{name: "prefix left space", input: " <foo cat", stdout: "foocontent\n"},
		{name: "prefix space", input: " < foo cat", stdout: "foocontent\n"},
		{name: "prefix default fd no space", input: "0<foo cat", stdout: "foocontent\n"},
		{name: "prefix default fd right space", input: "0< foo cat", stdout: "foocontent\n"},
		{name: "prefix default fd left space", input: " 0<foo cat", stdout: "foocontent\n"},
		{name: "prefix default fd space", input: " 0< foo cat", stdout: "foocontent\n"},
		{name: "prefix fd 9 no space", input: "9<foo cat " + selfFD() + "/9", stdout: "foocontent\n"},
		{name: "prefix fd 9 right space", input: "9< foo cat " + selfFD() + "/9", stdout: "foocontent\n"},
		{name: "prefix fd 9 left space", input: " 9<foo cat " + selfFD() + "/9", stdout: "foocontent\n"},
		{name: "prefix fd 9 space", input: " 9< foo cat " + selfFD() + "/9", stdout: "foocontent\n"},

		{name: "prefix suffix fd 9", input: " 9< foo 8<foo cat " + selfFD() + "/9 < foo 7<foo", stdout: "foocontent\n"},

		{name: "prefix single quote no space", input: "<'foo' cat", stdout: "foocontent\n"},
		{name: "prefix single quote space", input: " < 'foo' cat", stdout: "foocontent\n"},
		{name: "prefix double quote no space", input: "<\"foo\" cat", stdout: "foocontent\n"},
		{name: "prefix double quote space", input: " < \"foo\" cat", stdout: "foocontent\n"},
		// TODO: Handle this.
		// {name: "prefix mixed quote", input: "<\"f\"o'o' cat", stdout: "foocontent\n"},
		// {name: "prefix subst no space", input: "<$(echo foo) cat", stdout: "foocontent\n"},
		// {name: "prefix subst space", input: " < $(echo foo) cat", stdout: "foocontent\n"},
		// {name: "prefix backtick no space", input: "<`echo foo` cat", stdout: "foocontent\n"},
		// {name: "prefix backtick space", input: " < `echo foo` cat", stdout: "foocontent\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, run(tt))
	}
}

// NOTE: These tests can't be run in parallel because they modify the environment, cwd, and other global state.
func run(tt testCase) func(t *testing.T) {
	return func(t *testing.T) {
		t.Helper()

		shellList := []string{"sh", "sh -c", "bash", "bash -c", "bash --posix", "bash --posix -c", "zsh", "zsh -c", "gosh2"}
		if testing.Short() {
			shellList = []string{"bash --posix", "gosh2"}
		}
		for _, elem := range shellList {
			t.Run(elem, func(t *testing.T) {
				stdout := bytes.NewBuffer(nil)
				stderr := bytes.NewBuffer(nil)
				setupEnv(t)
				defer func() { require.Nil(t, recover(), "panic") }()
				var exitCode int
				var err error
				switch elem {
				case "gosh2":
					exitCode, err = parser.Run(strings.NewReader(tt.input), nil, stdout, stderr)
				default:
					parts := strings.Split(elem, " ")
					if _, err := exec.LookPath(parts[0]); err != nil {
						t.Skipf("skipping test %q: %s", elem, err)
					}
					if strings.HasSuffix(elem, "-c") {
						parts = append(parts, tt.input)
						cmd := exec.Command(parts[0], parts[1:]...)
						cmd.Stdout = stdout
						cmd.Stderr = stderr
						var e0 *exec.ExitError
						if err := cmd.Run(); err != nil && !errors.As(err, &e0) {
							require.NoError(t, err, "failed to run command %q", elem)
						}
						require.NotNil(t, cmd.ProcessState, "ProcessState is nil")
						exitCode = cmd.ProcessState.ExitCode()
					} else {
						cmd := exec.Command(parts[0], parts[1:]...)
						cmd.Stdin = strings.NewReader(tt.input)
						cmd.Stdout = stdout
						cmd.Stderr = stderr
						var e0 *exec.ExitError
						if err := cmd.Run(); err != nil && !errors.As(err, &e0) {
							require.NoError(t, err, "failed to run command %q", elem)
						}
						require.NotNil(t, cmd.ProcessState, "ProcessState is nil")
						exitCode = cmd.ProcessState.ExitCode()
					}
				}
				if tt.wantErr {
					require.Error(t, err, "parser.Run didn't fail but should have")
				} else {
					require.NoError(t, err, "parser.Run failed")
				}
				if !assert.Equal(t, tt.exitCode, exitCode, "Exit code mismatch") {
					t.Logf("Stderr: %s", stderr)
				}
				require.Equal(t, tt.stdout, stdout.String(), "Stdout mismatch")
				if tt.stderr != "" {
					require.Equal(t, tt.stderr, stderr.String(), "Stderr mismatch")
				}
			})
		}
	}
}

/*
	//? ?
 	// input = "foo.sh 7<foo | cat -e; echo --; ls; echo --; cat a"


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
	input = "(echo hello) > foo; cat foo"
	input = "echo hello; cat foo"
*/
