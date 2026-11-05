package main

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GyeongHoKim/build-your-own-coreutils/internal/filetime"
)

var (
	now        = time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	seedAccess = time.Date(2001, 1, 1, 1, 1, 1, 0, time.UTC)
	seedModify = time.Date(2002, 2, 2, 2, 2, 2, 0, time.UTC)
	refAccess  = time.Date(2011, 3, 4, 5, 6, 7, 0, time.UTC)
	refModify  = time.Date(2012, 8, 9, 10, 11, 12, 0, time.UTC)
	stamp      = time.Date(2026, 11, 6, 14, 30, 15, 0, time.UTC)
)

type invocation struct {
	status int
	stdout string
	stderr string
}

func invoke(t *testing.T, args ...string) invocation {
	t.Helper()
	var stdout, stderr bytes.Buffer
	status := runAt(args, &stdout, &stderr, now)
	return invocation{status, stdout.String(), stderr.String()}
}

func inTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	return dir
}

func writeSeeded(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, seedAccess, seedModify); err != nil {
		t.Fatal(err)
	}
}

func writeReference(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, refAccess, refModify); err != nil {
		t.Fatal(err)
	}
}

func readTimes(t *testing.T, path string) filetime.Times {
	t.Helper()
	times, err := filetime.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	return times
}

func assertTimes(t *testing.T, path string, access, modify time.Time) {
	t.Helper()
	got := readTimes(t, path)
	if !got.Access.Equal(access) || !got.Modify.Equal(modify) {
		t.Errorf("%s: got access=%s modify=%s, want access=%s modify=%s", path, got.Access, got.Modify, access, modify)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s: got %v, want it to be absent", path, err)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Errorf("%s: %v", path, err)
	}
}

func assertSuccess(t *testing.T, got invocation) {
	t.Helper()
	if got.status != 0 || got.stdout != "" || got.stderr != "" {
		t.Fatalf("got status %d, stdout %q, stderr %q; want status 0 and no output", got.status, got.stdout, got.stderr)
	}
}

func assertFailure(t *testing.T, got invocation, status int) {
	t.Helper()
	if got.status != status || got.stdout != "" || !strings.HasPrefix(got.stderr, "touch: ") {
		t.Fatalf("got status %d, stdout %q, stderr %q; want status %d, empty stdout, and a touch: diagnostic",
			got.status, got.stdout, got.stderr, status)
	}
}

func TestRunArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no operands", nil},
		{"only options", []string{"-a", "-m"}},
		{"unknown option", []string{"-z", "target"}},
		{"combined options", []string{"-am", "target"}},
		{"missing -r value", []string{"-r"}},
		{"missing -t value", []string{"-t"}},
		{"empty -r value", []string{"-r", "", "target"}},
		{"empty -t value", []string{"-t", "", "target"}},
		{"-r and -t", []string{"-r", "ref", "-t", "202611061430", "target"}},
		{"-t and -r", []string{"-t", "202611061430", "-r", "ref", "target"}},
		{"invalid date", []string{"-t", "2026110614", "target"}},
		{"invalid calendar date", []string{"-t", "202602300900", "target"}},
		{"leap second", []string{"-t", "202611061430.60", "target"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inTempDir(t)
			writeReference(t, "ref")
			got := invoke(t, tt.args...)
			assertFailure(t, got, 2)
			if !strings.Contains(got.stderr, "usage: touch") {
				t.Errorf("stderr %q does not show usage", got.stderr)
			}
			assertMissing(t, "target")
		})
	}
}

func TestRunPreflight(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		status int
	}{
		{"missing reference", []string{"-r", "missing", "existing", "new"}, 1},
		{"missing reference with -c", []string{"-c", "-r", "missing", "existing", "new"}, 1},
		{"invalid date", []string{"-t", "20261106", "existing", "new"}, 2},
		{"conflicting sources", []string{"-r", "existing", "-t", "202611061430", "existing", "new"}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inTempDir(t)
			writeSeeded(t, "existing")
			assertFailure(t, invoke(t, tt.args...), tt.status)
			assertTimes(t, "existing", seedAccess, seedModify)
			assertMissing(t, "new")
		})
	}
}

func TestRunSelection(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantAccess time.Time
		wantModify time.Time
	}{
		{"default", nil, now, now},
		{"-a", []string{"-a"}, now, seedModify},
		{"-m", []string{"-m"}, seedAccess, now},
		{"-a -m", []string{"-a", "-m"}, now, now},
		{"-m -a", []string{"-m", "-a"}, now, now},
		{"-t", []string{"-t", "202611061430.15"}, stamp, stamp},
		{"-a -t", []string{"-a", "-t", "202611061430.15"}, stamp, seedModify},
		{"-m -t", []string{"-m", "-t", "202611061430.15"}, seedAccess, stamp},
		{"-r", []string{"-r", "ref"}, refAccess, refModify},
		{"-a -r", []string{"-a", "-r", "ref"}, refAccess, seedModify},
		{"-m -r", []string{"-m", "-r", "ref"}, seedAccess, refModify},
		{"-c", []string{"-c"}, now, now},
		{"repeated -t", []string{"-t", "200001010000", "-t", "202611061430.15"}, stamp, stamp},
		{"repeated -r", []string{"-r", "missing", "-r", "ref"}, refAccess, refModify},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inTempDir(t)
			writeSeeded(t, "target")
			writeReference(t, "ref")
			assertSuccess(t, invoke(t, append(tt.args, "target")...))
			assertTimes(t, "target", tt.wantAccess, tt.wantModify)
			content, err := os.ReadFile("target")
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != "content" {
				t.Errorf("got content %q, want %q", content, "content")
			}
		})
	}
}

func TestRunCreates(t *testing.T) {
	inTempDir(t)
	writeReference(t, "ref")
	if err := os.Mkdir("dir", 0o700); err != nil {
		t.Fatal(err)
	}
	assertSuccess(t, invoke(t, "-r", "ref", "one", "dir", "two"))
	for _, path := range []string{"one", "dir", "two"} {
		assertTimes(t, path, refAccess, refModify)
	}
	info, err := os.Stat("one")
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Size() != 0 {
		t.Errorf("got mode %s size %d, want an empty regular file", info.Mode(), info.Size())
	}
}

func TestRunNoCreate(t *testing.T) {
	inTempDir(t)
	writeSeeded(t, "existing")
	assertSuccess(t, invoke(t, "-c", "-t", "202611061430.15", "missing", filepath.Join("absent", "child"), "existing"))
	assertMissing(t, "missing")
	assertMissing(t, "absent")
	assertTimes(t, "existing", stamp, stamp)
}

func TestRunSelfReference(t *testing.T) {
	inTempDir(t)
	writeReference(t, "ref")
	writeSeeded(t, "target")
	assertSuccess(t, invoke(t, "-r", "ref", "target", "ref"))
	assertTimes(t, "target", refAccess, refModify)
	assertTimes(t, "ref", refAccess, refModify)
}

func TestRunOperands(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		created []string
	}{
		{"double dash", []string{"--", "-a"}, []string{"-a"}},
		{"single dash", []string{"-"}, []string{"-"}},
		{"option after operand", []string{"first", "-m"}, []string{"first", "-m"}},
		{"help after operand", []string{"first", "--help"}, []string{"first", "--help"}},
		{"space and non-ASCII", []string{filepath.Join("sample files", "보고서.txt")}, []string{filepath.Join("sample files", "보고서.txt")}},
		{"repeated operand", []string{"same", "same"}, []string{"same"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inTempDir(t)
			if err := os.Mkdir("sample files", 0o700); err != nil {
				t.Fatal(err)
			}
			assertSuccess(t, invoke(t, tt.args...))
			for _, path := range tt.created {
				assertExists(t, path)
				assertTimes(t, path, now, now)
			}
		})
	}
}

func TestRunContinues(t *testing.T) {
	inTempDir(t)
	writeSeeded(t, "regular")
	got := invoke(t, filepath.Join("missing", "child"), filepath.Join("regular", "child"), "after")
	assertFailure(t, got, 1)
	for _, path := range []string{filepath.Join("missing", "child"), filepath.Join("regular", "child")} {
		if !strings.Contains(got.stderr, path) {
			t.Errorf("stderr %q does not mention %s", got.stderr, path)
		}
	}
	if lines := strings.Count(got.stderr, "\n"); lines != 2 {
		t.Errorf("got %d diagnostic lines, want 2:\n%s", lines, got.stderr)
	}
	assertExists(t, "after")
	assertTimes(t, "after", now, now)
}

func TestRunNoCreateFileParent(t *testing.T) {
	inTempDir(t)
	writeSeeded(t, "regular")
	assertFailure(t, invoke(t, "-c", filepath.Join("regular", "child")), 1)
}

func TestRunLinks(t *testing.T) {
	t.Run("target and reference", func(t *testing.T) {
		inTempDir(t)
		writeSeeded(t, "target")
		writeReference(t, "ref")
		symlink(t, "target", "target-link")
		symlink(t, "ref", "ref-link")
		assertSuccess(t, invoke(t, "-r", "ref-link", "target-link"))
		assertTimes(t, "target", refAccess, refModify)
	})

	t.Run("dangling", func(t *testing.T) {
		inTempDir(t)
		symlink(t, "created", "link")
		assertSuccess(t, invoke(t, "link"))
		assertTimes(t, "created", now, now)
	})

	t.Run("dangling with -c", func(t *testing.T) {
		inTempDir(t)
		symlink(t, "created", "link")
		assertSuccess(t, invoke(t, "-c", "link"))
		assertMissing(t, "created")
	})
}

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create symbolic link: %v", err)
	}
}

func TestRunHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-help"}, {"-h"}, {"--h"}, {"-a", "--help", "target"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			inTempDir(t)
			got := invoke(t, args...)
			if got.status != 0 || got.stderr != "" || !strings.HasPrefix(got.stdout, "usage: touch") {
				t.Fatalf("got status %d, stdout %q, stderr %q; want help on stdout", got.status, got.stdout, got.stderr)
			}
			assertMissing(t, "target")
		})
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestRunHelpOutputFailure(t *testing.T) {
	var stderr bytes.Buffer
	if got := runAt([]string{"--help"}, failingWriter{}, &stderr, now); got != 1 {
		t.Fatalf("got status %d, want 1", got)
	}
	if !strings.HasPrefix(stderr.String(), "touch: ") {
		t.Errorf("got stderr %q, want a touch: diagnostic", stderr.String())
	}
}

func TestRunCapturesCurrentTime(t *testing.T) {
	inTempDir(t)
	start := time.Now()
	var stdout, stderr bytes.Buffer
	if got := run([]string{"one", "two"}, &stdout, &stderr); got != 0 {
		t.Fatalf("got status %d, stderr %q", got, stderr.String())
	}
	end := time.Now()
	one, two := readTimes(t, "one"), readTimes(t, "two")
	if !one.Modify.Equal(two.Modify) || !one.Access.Equal(two.Access) || !one.Access.Equal(one.Modify) {
		t.Errorf("got one=%+v two=%+v, want one shared timestamp", one, two)
	}
	if one.Modify.Before(start.Truncate(time.Second)) || one.Modify.After(end) {
		t.Errorf("got %s, want between %s and %s", one.Modify, start, end)
	}
}
