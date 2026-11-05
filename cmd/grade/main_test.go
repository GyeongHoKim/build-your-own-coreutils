package main

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

// fakeModeEnv makes the test binary behave as a fake tree executable, so run
// can be exercised end to end without a real implementation.
const fakeModeEnv = "GRADE_TEST_FAKE_TREE"

const (
	fakeCorrect = "correct" // matches every case
	fakeWrong   = "wrong"   // prints the wrong output for every case
	fakeSilent  = "silent"  // fails invalid options without a message on stderr
)

func TestMain(m *testing.M) {
	if mode := os.Getenv(fakeModeEnv); mode != "" {
		os.Exit(runFakeTree(mode, os.Args[1:]))
	}
	if mode := os.Getenv(processHelperEnv); mode != "" {
		os.Exit(runProcessHelper(mode, os.Args[1:]))
	}
	os.Exit(m.Run())
}

func runFakeTree(mode string, args []string) int {
	for _, c := range treeCases {
		if !slices.Equal(c.args, args) {
			continue
		}
		switch {
		case mode == fakeWrong:
			fmt.Println("wrong")
			return 0
		case c.fail && mode == fakeSilent:
			return 1
		case c.fail:
			fmt.Fprintln(os.Stderr, "tree: invalid option")
			return 1 // any non-zero status is accepted
		default:
			fmt.Print(c.want)
			return 0
		}
	}
	return 3
}

func TestRunRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no arguments", nil},
		{"unknown command", []string{"unknown", "missing"}},
		{"missing executable", []string{"tree", t.TempDir() + "/missing"}},
		{"directory", []string{"tree", t.TempDir()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != 2 {
				t.Fatalf("got status %d, want 2", got)
			}
			if stderr.Len() == 0 {
				t.Error("want a message on stderr")
			}
		})
	}
}

func TestRunGradesExecutable(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		wantStatus int
		wantOutput []string
	}{
		{"correct", fakeCorrect, 0, []string{"PASS default", "PASS hidden files", "PASS invalid option", "3/3 passed"}},
		{"wrong output", fakeWrong, 1, []string{"FAIL default", "FAIL hidden files", "FAIL invalid option", "0/3 passed"}},
		{"missing error message", fakeSilent, 1, []string{"PASS default", "PASS hidden files", "FAIL invalid option", "2/3 passed"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(fakeModeEnv, tt.mode)
			var stdout, stderr bytes.Buffer
			if got := run([]string{"tree", os.Args[0]}, &stdout, &stderr); got != tt.wantStatus {
				t.Fatalf("got status %d, want %d\nstdout:\n%s\nstderr:\n%s", got, tt.wantStatus, &stdout, &stderr)
			}
			for _, want := range tt.wantOutput {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("stdout does not contain %q:\n%s", want, &stdout)
				}
			}
		})
	}
}

func TestRunUsageListsGraders(t *testing.T) {
	var stdout, stderr bytes.Buffer
	run(nil, &stdout, &stderr)
	for name := range graders {
		if !strings.Contains(stderr.String(), name) {
			t.Errorf("usage %q does not list %s", stderr.String(), name)
		}
	}
}
