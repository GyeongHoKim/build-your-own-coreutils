// Command grade checks learner implementations against command behavior.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const caseTimeout = 10 * time.Second

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// testCase describes one invocation of the executable under test.
//
// A successful case must exit with status 0, print want to stdout, and leave
// stderr empty. A failing case must exit with a non-zero status, print nothing
// to stdout, and explain the problem on stderr. The exact non-zero status is
// not checked because tree(1) does not document one for invalid options.
type testCase struct {
	name string
	args []string
	want string
	fail bool
}

// treeCases are run inside the directory created by writeTreeFixture.
var treeCases = []testCase{
	{name: "default", args: []string{"."}, want: ".\n└── visible\n\n1 directory, 1 file\n"},
	{name: "hidden files", args: []string{"-a", "--noreport", "."}, want: ".\n├── .hidden\n└── visible\n"},
	{name: "invalid option", args: []string{"-z"}, fail: true},
}

// result is the observed behavior of one invocation.
type result struct {
	status   int
	stdout   string
	stderr   string
	timedOut bool
}

// passed reports whether r satisfies c.
func (c testCase) passed(r result) bool {
	if r.timedOut {
		return false
	}
	if c.fail {
		return r.status != 0 && r.stdout == "" && r.stderr != ""
	}
	return r.status == 0 && r.stdout == c.want && r.stderr == ""
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 || args[0] != "tree" {
		fmt.Fprintln(stderr, "usage: grade tree <executable>")
		return 2
	}
	executable, err := filepath.Abs(args[1])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	// The executable is explicitly selected by the user.
	if info, err := os.Stat(executable); err != nil || info.IsDir() { // #nosec G703 -- the user selects the executable to grade.
		fmt.Fprintf(stderr, "invalid executable: %s\n", executable)
		return 2
	}
	dir, err := os.MkdirTemp("", "tree-grade-")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintf(stderr, "warning: cannot remove %s: %v\n", dir, err)
		}
	}()
	if err := writeTreeFixture(dir); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	failures := 0
	for _, c := range treeCases {
		r, err := runCase(executable, dir, c)
		if err != nil {
			fmt.Fprintf(stderr, "cannot execute: %v\n", err)
			return 2
		}
		if c.passed(r) {
			fmt.Fprintf(stdout, "PASS %s\n", c.name)
			continue
		}
		failures++
		reportFailure(stdout, c, r)
	}
	fmt.Fprintf(stdout, "%d/%d passed\n", len(treeCases)-failures, len(treeCases))
	if failures > 0 {
		return 1
	}
	return 0
}

// writeTreeFixture creates the directory contents the tree cases expect.
func writeTreeFixture(dir string) error {
	for _, name := range []string{"visible", ".hidden"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			return err
		}
	}
	return nil
}

// runCase runs executable in dir with c's arguments. A non-zero exit status is
// part of the result; only a failure to start the process is returned as an
// error.
func runCase(executable, dir string, c testCase) (result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), caseTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, executable, c.args...) // #nosec G204 G702 -- grading the user-selected executable is the purpose of this command.
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	r := result{}
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			return result{}, err
		}
		r.status = exit.ExitCode()
		r.timedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
	}
	// Windows executables may use CRLF line endings.
	r.stdout = strings.ReplaceAll(stdout.String(), "\r\n", "\n")
	r.stderr = stderr.String()
	return r, nil
}

func reportFailure(w io.Writer, c testCase, r result) {
	if r.timedOut {
		fmt.Fprintf(w, "FAIL %s: timed out after %s\n", c.name, caseTimeout)
		return
	}
	if c.fail {
		fmt.Fprintf(w, "FAIL %s: want non-zero status, empty stdout and a message on stderr; got status %d, stdout %q, stderr %q\n",
			c.name, r.status, r.stdout, r.stderr)
		return
	}
	fmt.Fprintf(w, "FAIL %s: want status 0, stdout %q, empty stderr; got status %d, stdout %q, stderr %q\n",
		c.name, c.want, r.status, r.stdout, r.stderr)
}
