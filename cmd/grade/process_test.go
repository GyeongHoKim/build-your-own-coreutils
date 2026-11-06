package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const processHelperEnv = "GRADE_TEST_PROCESS_HELPER"

const (
	helperEchoArgs   = "echo-args"
	helperSleep      = "sleep"
	helperSpawnChild = "spawn-child"
)

func runProcessHelper(mode string, args []string) int {
	switch mode {
	case helperEchoArgs:
		for _, arg := range args {
			fmt.Printf("%q\n", arg)
		}
		return 0
	case helperSleep:
		time.Sleep(3 * time.Second)
		return 0
	case helperSpawnChild:
		child := exec.CommandContext(context.Background(), os.Args[0])
		child.Env = append(os.Environ(), processHelperEnv+"="+helperSleep)
		child.Dir = os.TempDir()
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Fprintln(os.Stderr, child.Process.Pid)
		return 0
	default:
		return 3
	}
}

func testExecutable(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func copyExecutable(t *testing.T, dst string) {
	t.Helper()
	src, err := os.Open(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, src); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestInvokeLiteralArguments(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "helper dir")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(dir, "grade helper.exe")
	copyExecutable(t, executable)
	t.Setenv(processHelperEnv, helperEchoArgs)

	args := []string{"보고서 file.txt", "a b", "", `"quoted"`, `back\slash\`, "-"}
	r, err := invoke(executable, t.TempDir(), args, caseTimeout)
	if err != nil {
		t.Fatal(err)
	}
	var want strings.Builder
	for _, arg := range args {
		fmt.Fprintf(&want, "%q\n", arg)
	}
	if r.status != 0 || r.stdout != want.String() {
		t.Fatalf("got status %d, stdout:\n%s\nwant:\n%s", r.status, r.stdout, want.String())
	}
	if r.started.IsZero() || r.finished.Before(r.started) {
		t.Errorf("got started %s finished %s, want an ordered interval", r.started, r.finished)
	}
}

func TestInvokeTimeout(t *testing.T) {
	t.Setenv(processHelperEnv, helperSleep)
	start := time.Now()
	r, err := invoke(testExecutable(t), t.TempDir(), nil, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !r.timedOut {
		t.Errorf("got %+v, want a timeout", r)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("invoke took %s, want it bounded by the timeout", elapsed)
	}
}

func TestInvokeInheritedPipe(t *testing.T) {
	t.Setenv(processHelperEnv, helperSpawnChild)
	start := time.Now()
	r, err := invoke(testExecutable(t), t.TempDir(), nil, caseTimeout)
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	killChild(t, r.stderr)
	if elapsed > 2*time.Second {
		t.Errorf("invoke took %s, want it bounded by the wait delay", elapsed)
	}
	if !r.timedOut {
		t.Errorf("got %+v, want open inherited pipes to count as a timeout", r)
	}
}

func killChild(t *testing.T, stderr string) {
	t.Helper()
	pid, err := strconv.Atoi(strings.TrimSpace(stderr))
	if err != nil {
		t.Fatalf("helper did not report its child: %q", stderr)
	}
	child, err := os.FindProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		t.Error(err)
	}
}

func TestInvokeStartupFailure(t *testing.T) {
	if _, err := invoke(filepath.Join(t.TempDir(), "missing"), t.TempDir(), nil, caseTimeout); err == nil {
		t.Fatal("want an error for a missing executable")
	}
}
