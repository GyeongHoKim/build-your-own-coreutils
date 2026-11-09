package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// rmMode says what an rm case expects on the standard streams.
type rmMode int

const (
	rmQuiet   rmMode = iota // status 0, no output
	rmError                 // non-zero status and a diagnostic naming each of mention
	rmPrompt                // status 0 and a prompt on stderr
	rmVerbose               // status 0 and a stdout line naming each of mention
)

// rmInvocation is one prepared rm run and the files it must leave behind.
type rmInvocation struct {
	args    []string
	stdin   string
	cwd     string // working directory relative to the fixture; empty for its root
	mode    rmMode
	mention []string // text the diagnostic or -v line must contain
	gone    []string // paths, relative to the fixture, that must not exist afterwards
	kept    []string // paths, relative to the fixture, that must still exist
}

type rmCase struct {
	name    string
	tier    string
	summary string
	prepare func(dir string) (rmInvocation, string, error)
}

func gradeRm(executable string, stdout, stderr io.Writer) int {
	out := &reportWriter{w: stdout}
	outcomes, graderErr := runRmCases(executable, caseTimeout, out)
	status := summarize(out, stderr, outcomes, graderErr)
	if out.err != nil {
		fmt.Fprintf(stderr, "cannot write results: %v\n", out.err)
		return 2
	}
	return status
}

func runRmCases(executable string, timeout time.Duration, out io.Writer) ([]caseOutcome, error) {
	var outcomes []caseOutcome
	for _, c := range rmCases() {
		o, err := runRmCase(c, executable, timeout)
		if err != nil {
			return outcomes, fmt.Errorf("case %s: %w", c.name, err)
		}
		outcomes = append(outcomes, o)
		switch o.kind {
		case outcomePass:
			fmt.Fprintf(out, "PASS %s\n", o.name)
		case outcomeFail:
			fmt.Fprintf(out, "FAIL %s: %s\n", o.name, o.message)
		case outcomeSkip:
			fmt.Fprintf(out, "SKIP %s: %s\n", o.name, o.message)
		}
	}
	return outcomes, nil
}

func runRmCase(c rmCase, executable string, timeout time.Duration) (o caseOutcome, err error) {
	dir, err := os.MkdirTemp("", "rm-grade-")
	if err != nil {
		return o, fmt.Errorf("create fixture directory: %w", err)
	}
	defer func() {
		if rmErr := os.RemoveAll(dir); rmErr != nil {
			err = errors.Join(err, fmt.Errorf("cannot remove %s: %w", dir, rmErr))
		}
	}()

	o = caseOutcome{name: c.name, tier: c.tier}
	inv, skip, err := c.prepare(dir)
	if err != nil {
		return o, fmt.Errorf("prepare fixture: %w", err)
	}
	if skip != "" {
		o.kind, o.message = outcomeSkip, skip
		return o, nil
	}
	r, err := invokeInput(executable, filepath.Join(dir, inv.cwd), inv.args, inv.stdin, timeout)
	if err != nil {
		return o, fmt.Errorf("cannot execute: %w", err)
	}
	if failure := inv.judge(&r, dir, timeout); failure != "" {
		o.kind, o.message = outcomeFail, fmt.Sprintf("%s; args %q: %s", c.summary, inv.args, failure)
	}
	return o, nil
}

func (inv *rmInvocation) judge(r *result, dir string, timeout time.Duration) string {
	switch {
	case r.timedOut:
		return fmt.Sprintf("timed out after %s", timeout)
	case inv.mode == rmError && (r.status == 0 || r.stdout != "" || r.stderr == ""):
		return fmt.Sprintf("want non-zero status, empty stdout and a message on stderr; got status %d, stdout %q, stderr %q",
			r.status, r.stdout, r.stderr)
	case inv.mode == rmError:
		for _, m := range inv.mention {
			if f := pathDiagnosticFailure(r.stderr, m); f != "" {
				return f
			}
		}
	case inv.mode == rmQuiet && (r.status != 0 || r.stdout != "" || r.stderr != ""):
		return fmt.Sprintf("want status 0 and no output; got status %d, stdout %q, stderr %q", r.status, r.stdout, r.stderr)
	case inv.mode == rmPrompt && (r.status != 0 || r.stdout != "" || r.stderr == ""):
		return fmt.Sprintf("want status 0, empty stdout and a prompt on stderr; got status %d, stdout %q, stderr %q",
			r.status, r.stdout, r.stderr)
	case inv.mode == rmVerbose && (r.status != 0 || r.stderr != ""):
		return fmt.Sprintf("want status 0 and empty stderr; got status %d, stderr %q", r.status, r.stderr)
	case inv.mode == rmVerbose:
		for _, m := range inv.mention {
			if !strings.Contains(r.stdout, m) {
				return fmt.Sprintf("stdout %q does not mention %s", r.stdout, m)
			}
		}
	}
	for _, p := range inv.gone {
		_, err := os.Lstat(filepath.Join(dir, p))
		switch {
		case err == nil:
			return p + " still exists"
		case !errors.Is(err, fs.ErrNotExist):
			return fmt.Sprintf("cannot tell whether %s was removed: %v", p, err)
		}
	}
	for _, p := range inv.kept {
		_, err := os.Lstat(filepath.Join(dir, p))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return p + " was removed"
		case err != nil:
			return fmt.Sprintf("cannot tell whether %s was kept: %v", p, err)
		}
	}
	return ""
}

// rmFixture writes files and directories below dir. A name ending in "/" is a
// directory.
func rmFixture(dir string, names ...string) error {
	for _, n := range names {
		p := filepath.Join(dir, filepath.FromSlash(n))
		if strings.HasSuffix(n, "/") {
			if err := os.MkdirAll(p, 0o700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(fileContent), 0o600); err != nil {
			return err
		}
	}
	return nil
}

// rmSimple builds a case that needs only a fixture and a fixed invocation.
func rmSimple(name, tier, summary string, files []string, inv *rmInvocation) rmCase {
	return rmCase{
		name: name, tier: tier, summary: summary,
		prepare: func(dir string) (rmInvocation, string, error) {
			return *inv, "", rmFixture(dir, files...)
		},
	}
}

var rmTree = []string{"tree/", "tree/a.txt", "tree/sub/", "tree/sub/b.txt", "tree/empty/"}

func rmCases() []rmCase {
	return []rmCase{
		rmSimple("remove-file", tierCore, "removes a file",
			[]string{"victim.txt", "other.txt"},
			&rmInvocation{args: []string{"victim.txt"}, gone: []string{"victim.txt"}, kept: []string{"other.txt"}}),
		rmSimple("multiple-operands", tierCore, "removes every operand",
			[]string{"one.txt", "two.txt", "three.txt"},
			&rmInvocation{args: []string{"one.txt", "two.txt", "three.txt"}, gone: []string{"one.txt", "two.txt", "three.txt"}}),
		rmSimple("missing-operand", tierCore, "reports a missing operand, fails, and still removes the others",
			[]string{"first.txt", "last.txt"},
			&rmInvocation{
				args: []string{"first.txt", "absent.txt", "last.txt"}, mode: rmError, mention: []string{"absent.txt"},
				gone: []string{"first.txt", "last.txt"},
			}),
		rmSimple("force-missing-operand", tierCore, "-f ignores a missing operand silently",
			[]string{"present.txt"},
			&rmInvocation{args: []string{"-f", "absent.txt", "present.txt"}, gone: []string{"present.txt"}}),
		rmSimple("directory-without-r", tierCore, "refuses a directory without -r and keeps it",
			[]string{"dir/", "dir/file.txt"},
			&rmInvocation{args: []string{"dir"}, mode: rmError, mention: []string{"dir"}, kept: []string{"dir", "dir/file.txt"}}),
		rmSimple("recursive-tree", tierCore, "-r removes a directory tree",
			append([]string{"keep.txt"}, rmTree...),
			&rmInvocation{args: []string{"-r", "tree"}, gone: []string{"tree"}, kept: []string{"keep.txt"}}),
		rmSimple("recursive-upper-tree", tierCore, "-R removes a directory tree",
			append([]string{"keep.txt"}, rmTree...),
			&rmInvocation{args: []string{"-R", "tree"}, gone: []string{"tree"}, kept: []string{"keep.txt"}}),
		{
			name: "recursive-keeps-link-target", tier: tierCore,
			summary: "-r removes a symbolic link inside a tree without following it",
			prepare: func(dir string) (rmInvocation, string, error) {
				if err := rmFixture(dir, "outside/", "outside/precious.txt", "tree/", "tree/file.txt"); err != nil {
					return rmInvocation{}, "", err
				}
				if err := os.Symlink(filepath.Join(dir, "outside"), filepath.Join(dir, "tree", "link")); err != nil {
					return rmInvocation{}, fmt.Sprintf("cannot create symbolic links: %v", err), nil
				}
				return rmInvocation{
					args: []string{"-r", "tree"}, gone: []string{"tree"},
					kept: []string{"outside", "outside/precious.txt"},
				}, "", nil
			},
		},
		rmSimple("double-dash", tierCore, "-- ends options so -name is an operand",
			[]string{"-dash", "other.txt"},
			&rmInvocation{args: []string{"--", "-dash"}, gone: []string{"-dash"}, kept: []string{"other.txt"}}),

		rmSimple("directory-flag-empty", tierExtended, "-d removes an empty directory",
			[]string{"empty/"},
			&rmInvocation{args: []string{"-d", "empty"}, gone: []string{"empty"}}),
		rmSimple("directory-flag-non-empty", tierExtended, "-d refuses a non-empty directory",
			[]string{"full/", "full/file.txt"},
			&rmInvocation{args: []string{"-d", "full"}, mode: rmError, mention: []string{"full"}, kept: []string{"full", "full/file.txt"}}),
		rmSimple("interactive-yes", tierExtended, "-i removes the file after a y answer and prompts on stderr",
			[]string{"victim.txt"},
			&rmInvocation{args: []string{"-i", "victim.txt"}, stdin: "y\n", mode: rmPrompt, gone: []string{"victim.txt"}}),
		rmSimple("interactive-no", tierExtended, "-i keeps the file after an n answer and still succeeds",
			[]string{"victim.txt"},
			&rmInvocation{args: []string{"-i", "victim.txt"}, stdin: "n\n", mode: rmPrompt, kept: []string{"victim.txt"}}),
		rmSimple("force-no-operands", tierExtended, "-f without operands succeeds silently",
			nil,
			&rmInvocation{args: []string{"-f"}}),
		rmSimple("refuse-dot-and-dotdot", tierExtended, "refuses . and .. and removes nothing",
			[]string{"work/", "work/file.txt", "sibling.txt"},
			&rmInvocation{
				args: []string{"-r", ".", ".."}, cwd: "work", mode: rmError, mention: []string{"."},
				kept: []string{"work/file.txt", "sibling.txt"},
			}),
		rmSimple("grouped-recursive-force", tierExtended, "-rf groups options and ignores a missing operand",
			append([]string{"keep.txt"}, rmTree...),
			&rmInvocation{args: []string{"-rf", "tree", "absent"}, gone: []string{"tree"}, kept: []string{"keep.txt"}}),
		rmSimple("verbose", tierExtended, "-v prints a line naming the removed path",
			[]string{"victim.txt"},
			&rmInvocation{args: []string{"-v", "victim.txt"}, mode: rmVerbose, mention: []string{"victim.txt"}, gone: []string{"victim.txt"}}),
	}
}
