package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func buildRm(t *testing.T) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "rm"+filepath.Ext(os.Args[0]))
	out, err := exec.CommandContext(t.Context(), "go", "build", "-o", exe, "github.com/GyeongHoKim/build-your-own-coreutils/cmd/rm").CombinedOutput() // #nosec G204 -- fixed arguments.
	if err != nil {
		t.Fatalf("build rm: %v\n%s", err, out)
	}
	return exe
}

func gradeRmExecutable(t *testing.T, exe string) (status int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	status = gradeRm(exe, &out, &errOut)
	return status, out.String(), errOut.String()
}

func TestGradeRmReference(t *testing.T) {
	status, stdout, stderr := gradeRmExecutable(t, buildRm(t))
	if status != 0 || stderr != "" {
		t.Fatalf("got status %d, stderr %q\n%s", status, stderr, stdout)
	}
	if strings.Contains(stdout, "FAIL ") {
		t.Errorf("unexpected failure:\n%s", stdout)
	}
	for _, c := range rmCases() {
		if !strings.Contains(stdout, "PASS "+c.name+"\n") && !strings.Contains(stdout, "SKIP "+c.name+":") {
			t.Errorf("case %s did not pass:\n%s", c.name, stdout)
		}
	}
}

func TestGradeRmNoopStub(t *testing.T) {
	// A program that exits 0 and removes nothing.
	exe := buildStub(t, "package main\n\nfunc main() {}\n")
	status, stdout, _ := gradeRmExecutable(t, exe)
	if status != 1 {
		t.Fatalf("got status %d, want 1\n%s", status, stdout)
	}
	for _, name := range []string{"remove-file", "multiple-operands", "missing-operand", "directory-without-r", "recursive-tree", "recursive-upper-tree", "double-dash"} {
		if !strings.Contains(stdout, "FAIL "+name+":") {
			t.Errorf("core case %s did not fail:\n%s", name, stdout)
		}
	}
}

func TestRmCaseNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range rmCases() {
		if seen[c.name] || c.summary == "" {
			t.Errorf("case %q duplicated or without summary", c.name)
		}
		seen[c.name] = true
		if c.tier != tierCore && c.tier != tierExtended {
			t.Errorf("case %s has tier %q", c.name, c.tier)
		}
	}
}

func buildStub(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "stub"+filepath.Ext(os.Args[0]))
	if out, err := exec.CommandContext(t.Context(), "go", "build", "-o", exe, src).CombinedOutput(); err != nil { // #nosec G204 -- fixed arguments.
		t.Fatalf("build stub: %v\n%s", err, out)
	}
	return exe
}

// A path that cannot be examined is neither gone nor kept; the failure says
// why instead of guessing.
func TestRmJudgeStatError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows reports a path under a file as missing")
	}
	dir := t.TempDir()
	if err := rmFixture(dir, "f"); err != nil {
		t.Fatal(err)
	}
	under := filepath.Join("f", "x") // ENOTDIR, not ENOENT
	for _, inv := range []rmInvocation{{gone: []string{under}}, {kept: []string{under}}} {
		got := inv.judge(&result{}, dir, 0)
		if !strings.Contains(got, under) || !strings.Contains(got, "not a directory") {
			t.Errorf("judge(gone %v, kept %v) = %q, want the path and its error", inv.gone, inv.kept, got)
		}
	}
}
