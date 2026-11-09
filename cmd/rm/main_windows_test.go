package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestRunLockedFile(t *testing.T) {
	tempDir(t)
	if err := os.Mkdir("d", 0o755); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join("d", "locked")
	writeFile(t, locked)
	writeFile(t, "other")
	p, err := syscall.UTF16PtrFromString(locked)
	if err != nil {
		t.Fatal(err)
	}
	// Without FILE_SHARE_DELETE, nobody can delete the file while this
	// handle is open. It is closed before the temporary directory is removed.
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := syscall.CloseHandle(h); err != nil {
			t.Error(err)
		}
	})

	status, _, stderr := invoke(t, "", "-r", "d", "other")
	if status != 1 {
		t.Errorf("status = %d, want 1", status)
	}
	lines := strings.SplitAfter(stderr, "\n")
	if len(lines) != 3 || lines[2] != "" ||
		!strings.HasPrefix(lines[0], `rm: cannot remove 'd\locked': `) ||
		!strings.HasPrefix(lines[1], "rm: cannot remove 'd': ") {
		t.Errorf("stderr = %q", stderr)
	}
	if exists("other") {
		t.Error("other still exists")
	}
}

func TestRunReadOnlyFilePrompt(t *testing.T) {
	old := stdinIsTerminal
	stdinIsTerminal = func() bool { return true }
	t.Cleanup(func() { stdinIsTerminal = old })

	tempDir(t)
	writeFile(t, "f")
	p, err := syscall.UTF16PtrFromString("f")
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.SetFileAttributes(p, syscall.FILE_ATTRIBUTE_READONLY); err != nil {
		t.Fatal(err)
	}

	status, stdout, stderr := invoke(t, "n\n", "f")
	if want := "rm: remove write-protected regular empty file 'f'? "; status != 0 || stdout != "" || stderr != want {
		t.Errorf("got %d %q %q, want 0 \"\" %q", status, stdout, stderr, want)
	}
	if !exists("f") {
		t.Fatal("f was removed after answer n")
	}

	status, stdout, stderr = invoke(t, "", "-f", "f")
	if status != 0 || stdout != "" || stderr != "" {
		t.Errorf("-f: got %d %q %q", status, stdout, stderr)
	}
	if exists("f") {
		t.Error("f still exists after -f")
	}
}
