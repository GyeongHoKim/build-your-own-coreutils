package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/GyeongHoKim/build-your-own-coreutils/internal/rm"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		opts     rm.Options
		operands []string
		help     bool
		err      string
	}{
		{name: "grouped", args: []string{"-rf", "a"}, opts: rm.Options{Recursive: true, Force: true}, operands: []string{"a"}},
		{name: "capital R", args: []string{"-R", "a"}, opts: rm.Options{Recursive: true}, operands: []string{"a"}},
		{name: "separate", args: []string{"-d", "-v", "a"}, opts: rm.Options{Dirs: true, Verbose: true}, operands: []string{"a"}},
		{name: "last wins interactive", args: []string{"-fi", "a"}, opts: rm.Options{Interactive: true}, operands: []string{"a"}},
		{name: "last wins force", args: []string{"-if", "a"}, opts: rm.Options{Force: true}, operands: []string{"a"}},
		{name: "double dash", args: []string{"--", "-f"}, operands: []string{"-f"}},
		{name: "lone dash", args: []string{"-", "a"}, operands: []string{"-", "a"}},
		{name: "options stop at operand", args: []string{"a", "-f"}, operands: []string{"a", "-f"}},
		{name: "unknown", args: []string{"-x"}, err: "unknown option -x"},
		{name: "short help", args: []string{"-h"}, help: true},
		{name: "long help", args: []string{"--help"}, help: true},
		{name: "unknown long", args: []string{"--force"}, err: "unknown option --force"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, operands, help, err := parseArgs(tt.args)
			if tt.err != "" {
				if err == nil || err.Error() != tt.err {
					t.Fatalf("err = %v, want %q", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if opts != tt.opts {
				t.Errorf("opts = %+v, want %+v", opts, tt.opts)
			}
			if !reflect.DeepEqual(operands, tt.operands) && (len(operands) != 0 || len(tt.operands) != 0) {
				t.Errorf("operands = %q, want %q", operands, tt.operands)
			}
			if help != tt.help {
				t.Errorf("help = %v, want %v", help, tt.help)
			}
		})
	}
}

func TestRunUsage(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		status     int
		wantStdout string // prefix
		wantStderr string // suffix
		silent     bool
	}{
		{name: "no operands", args: nil, status: 2, wantStderr: usage},
		{name: "force alone", args: []string{"-f"}, status: 0, silent: true},
		{name: "unknown option", args: []string{"-z"}, status: 2, wantStderr: usage},
		{name: "help", args: []string{"--help"}, status: 0, wantStdout: "usage"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			status := run(tt.args, strings.NewReader(""), &stdout, &stderr)
			if status != tt.status {
				t.Errorf("status = %d, want %d", status, tt.status)
			}
			if !strings.HasPrefix(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want prefix %q", stdout.String(), tt.wantStdout)
			}
			if !strings.HasSuffix(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want suffix %q", stderr.String(), tt.wantStderr)
			}
			if tt.silent && (stdout.Len() != 0 || stderr.Len() != 0) {
				t.Errorf("unexpected output %q %q", stdout.String(), stderr.String())
			}
		})
	}
}

// invoke runs rm in a fresh temporary directory with the given standard
// input and returns its status and output.
func invoke(t *testing.T, stdin string, args ...string) (status int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	status = run(args, strings.NewReader(stdin), &out, &errOut)
	return status, out.String(), errOut.String()
}

func tempDir(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestRunRemovesAndVerbose(t *testing.T) {
	tempDir(t)
	writeFile(t, "f")
	status, stdout, stderr := invoke(t, "", "-v", "f")
	if status != 0 || stdout != "removed 'f'\n" || stderr != "" {
		t.Errorf("got %d %q %q", status, stdout, stderr)
	}
	if exists("f") {
		t.Error("f still exists")
	}
}

func TestRunVerboseDirectory(t *testing.T) {
	tempDir(t)
	if err := os.Mkdir("d", 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join("d", "x"))
	status, stdout, _ := invoke(t, "", "-rv", "d")
	if status != 0 {
		t.Errorf("status = %d", status)
	}
	for _, want := range []string{"removed '" + filepath.Join("d", "x") + "'\n", "removed directory 'd'\n"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout %q lacks %q", stdout, want)
		}
	}
}

func TestRunMissingWithoutForce(t *testing.T) {
	tempDir(t)
	writeFile(t, "f")
	status, _, stderr := invoke(t, "", "missing", "f")
	if status != 1 {
		t.Errorf("status = %d, want 1", status)
	}
	if !strings.HasPrefix(stderr, "rm: cannot remove 'missing': ") || !strings.HasSuffix(stderr, "\n") {
		t.Errorf("stderr = %q", stderr)
	}
	if exists("f") {
		t.Error("f still exists")
	}
}

func TestRunMissingWithForce(t *testing.T) {
	tempDir(t)
	writeFile(t, "f")
	status, stdout, stderr := invoke(t, "", "-f", "missing", "f")
	if status != 0 || stdout != "" || stderr != "" {
		t.Errorf("got %d %q %q", status, stdout, stderr)
	}
	if exists("f") {
		t.Error("f still exists")
	}
}

func TestRunPrompt(t *testing.T) {
	const question = "rm: remove regular empty file 'f'? "
	tests := []struct {
		name    string
		stdin   string
		removed bool
	}{
		{name: "no", stdin: "n\n"},
		{name: "yes", stdin: "yes\n", removed: true},
		{name: "empty input", stdin: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir(t)
			writeFile(t, "f")
			status, stdout, stderr := invoke(t, tt.stdin, "-i", "f")
			if status != 0 || stdout != "" || stderr != question {
				t.Errorf("got %d %q %q", status, stdout, stderr)
			}
			if exists("f") == tt.removed {
				t.Errorf("exists(f) = %v", !tt.removed)
			}
		})
	}
}

func TestRunNotTerminalNoPrompt(t *testing.T) {
	tempDir(t)
	writeFile(t, "f")
	if err := os.Chmod("f", 0o400); err != nil {
		t.Fatal(err)
	}
	old := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = old })

	status, _, stderr := invoke(t, "", "f")
	if status != 0 || stderr != "" {
		t.Errorf("got %d %q", status, stderr)
	}
	if exists("f") {
		t.Error("f still exists")
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

// A question that could not be written is never answered, even if standard
// input holds an affirmative answer.
func TestRunPromptWriteError(t *testing.T) {
	tempDir(t)
	writeFile(t, "a")
	writeFile(t, "b")
	status := run([]string{"-i", "a", "b"}, strings.NewReader("y\ny\n"), &bytes.Buffer{}, failWriter{})
	if status != 1 {
		t.Errorf("status = %d, want 1", status)
	}
	if !exists("a") || !exists("b") {
		t.Error("a file was removed without its question being shown")
	}
}

type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, errors.New("input broken") }

func TestRunPromptReadError(t *testing.T) {
	tempDir(t)
	writeFile(t, "a")
	writeFile(t, "b")
	var stderr bytes.Buffer
	status := run([]string{"-i", "a", "b"}, failReader{}, &bytes.Buffer{}, &stderr)
	if status != 1 {
		t.Errorf("status = %d, want 1", status)
	}
	if got := stderr.String(); strings.Count(got, "input broken") != 1 {
		t.Errorf("stderr = %q, want one read diagnostic", got)
	}
	if !exists("a") || !exists("b") {
		t.Error("a file was removed")
	}
}

func TestRunVerboseWriteError(t *testing.T) {
	tempDir(t)
	writeFile(t, "a")
	writeFile(t, "b")
	var stderr bytes.Buffer
	status := run([]string{"-v", "a", "b"}, strings.NewReader(""), failWriter{}, &stderr)
	if status != 1 {
		t.Errorf("status = %d, want 1", status)
	}
	if got := stderr.String(); strings.Count(got, "disk full") != 1 {
		t.Errorf("stderr = %q, want one write diagnostic", got)
	}
}
