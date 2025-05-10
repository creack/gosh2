package executor_test

import (
	"bytes"
	"flag"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.creack.net/gosh2/parser"
)

func setupEnv(t *testing.T) {
	tmpDir, err := os.MkdirTemp("/tmp", "gosh2-executor")
	require.NoError(t, err, "failed to create temp dir")
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) }) // Best effort cleanup.

	require.NoError(t, os.Chdir(tmpDir), "failed to change dir to temp dir %q", tmpDir)
	require.NoError(t, os.WriteFile("foo", []byte("foo\n"), 0644), "failed to write file %q", "foo")

	for _, name := range []string{
		"b", "bb", "a", "aa", "ast", "bara", "foo", "foo.sh", "go.mod", "go.sum", "lexer", "tmp", "sh",
	} {
		f, err := os.Create(name)
		require.NoError(t, err, "failed to create file %q", name)
		require.NoError(t, f.Close(), "failed to close file %q", name)
	}

	require.NoError(t, os.Setenv("GOSH2_TEST", "1"), "failed to set env GOSH2_TEST")
}

var flSub = flag.Bool("sub", false, "Run as subshell")

func TestMain(m *testing.M) {
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

func TestExecutor(t *testing.T) {
	// Run the tests.
	tests := []struct {
		name     string
		input    string
		stdout   string
		stderr   string
		wantErr  bool
		exitCode int
	}{
		{name: "empty", input: "", stdout: ""},
		{name: "simple cmd", input: "ls a aa", stdout: "a\naa\n"},
		{name: "simple cmd error", input: "ls /foo/bar/not/exist", exitCode: 1},
		{name: "simple builtin cmd", input: "echo hello", stdout: "hello\n"},
		{name: "cmd with redir", input: "ls a aa > foo; cat foo", stdout: "a\naa\n"},
		{name: "builtin double redirect", input: "echo hello >> foo; echo world >> foo; cat foo", stdout: "hello\nworld\n"},
		{name: "left redirect", input: "echo hello > foo; cat<foo", stdout: "hello\n"},
	}

	for _, tt := range tests {
		// NOTE: These tests can't be run in parallel because they modify the environment, cwd, and other global state.
		t.Run(tt.name, func(t *testing.T) {
			setupEnv(t)

			stdout := bytes.NewBuffer(nil)
			stderr := bytes.NewBuffer(nil)
			exitCode, err := parser.Run(strings.NewReader(tt.input), nil, stdout, stderr)
			if tt.wantErr {
				require.Error(t, err, "parser.Run didn't fail but should have")
			} else {
				require.NoError(t, err, "parser.Run failed")
			}
			assert.Equal(t, tt.exitCode, exitCode, "Exit code mismatch")
			require.Equal(t, tt.stdout, stdout.String(), "Stdout mismatch")
			if tt.stderr != "" {
				require.Equal(t, tt.stderr, stderr.String(), "Stderr mismatch")
			}
		})
	}
}

/*
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
	input = "(echo hello) > foo; cat foo"
	input = "echo hello; cat foo"
*/
