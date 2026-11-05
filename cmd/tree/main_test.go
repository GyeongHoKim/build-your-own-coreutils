package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runTree(t *testing.T, args ...string) (status int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	status = run(args, &out, &errb)
	return status, out.String(), errb.String()
}

func TestRunReport(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	status, out, _ := runTree(t, root)
	if status != 0 || !strings.HasSuffix(out, "\n1 directory, 1 file\n") {
		t.Errorf("status = %d, out = %q", status, out)
	}

	status, out, _ = runTree(t, "-noreport", root)
	if status != 0 || strings.Contains(out, "director") {
		t.Errorf("-noreport: status = %d, out = %q", status, out)
	}
}

func TestRunExitCodes(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"help", []string{"-h"}, 0},
		{"unknown flag", []string{"-z"}, 2},
		{"missing dir", []string{missing}, 1},
		{"level zero", []string{"-L", "0", "."}, 1},
		{"negative level", []string{"-L", "-1", "."}, 1},
		{"valid level", []string{"-L", "1", t.TempDir()}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if status, _, _ := runTree(t, tt.args...); status != tt.want {
				t.Errorf("status = %d, want %d", status, tt.want)
			}
		})
	}
}
