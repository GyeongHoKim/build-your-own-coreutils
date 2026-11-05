package tree

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"dir1/sub", "dir2"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{".hidden", "a.txt", "z.txt", "dir1/b.txt", "dir1/sub/c.txt"} {
		if err := os.WriteFile(filepath.Join(root, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func render(t *testing.T, root string, opts Options) (string, Counts) {
	t.Helper()
	var buf bytes.Buffer
	counts, err := Print(&buf, root, opts)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	return strings.Replace(buf.String(), root, "root", 1), counts
}

func TestPrint(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want string
		c    Counts
	}{
		{
			name: "default hides dotfiles",
			want: `root
├── a.txt
├── dir1
│   ├── b.txt
│   └── sub
│       └── c.txt
├── dir2
└── z.txt
`,
			c: Counts{Dirs: 4, Files: 4},
		},
		{
			name: "-a",
			opts: Options{All: true},
			want: `root
├── .hidden
├── a.txt
├── dir1
│   ├── b.txt
│   └── sub
│       └── c.txt
├── dir2
└── z.txt
`,
			c: Counts{Dirs: 4, Files: 5},
		},
		{
			name: "-d",
			opts: Options{DirsOnly: true},
			want: `root
├── dir1
│   └── sub
└── dir2
`,
			c: Counts{Dirs: 4},
		},
		{
			name: "-L 1",
			opts: Options{MaxDepth: 1},
			want: `root
├── a.txt
├── dir1
├── dir2
└── z.txt
`,
			c: Counts{Dirs: 3, Files: 2},
		},
		{
			name: "--dirsfirst",
			opts: Options{DirsFirst: true},
			want: `root
├── dir1
│   ├── sub
│   │   └── c.txt
│   └── b.txt
├── dir2
├── a.txt
└── z.txt
`,
			c: Counts{Dirs: 4, Files: 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, c := render(t, fixture(t), tt.opts)
			if got != tt.want {
				t.Errorf("output mismatch\n got:\n%s\nwant:\n%s", got, tt.want)
			}
			if c != tt.c {
				t.Errorf("counts = %+v, want %+v", c, tt.c)
			}
		})
	}
}

func TestPrintEmptyDir(t *testing.T) {
	got, c := render(t, t.TempDir(), Options{})
	if got != "root\n" || c != (Counts{}) {
		t.Errorf("got %q %+v", got, c)
	}
}

func TestPrintSymlinkNotFollowed(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "real", "f"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	got, _ := render(t, root, Options{})
	want := "root\n├── link -> real\n└── real\n    └── f\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPrintMissingRoot(t *testing.T) {
	var buf bytes.Buffer
	_, err := Print(&buf, filepath.Join(t.TempDir(), "nope"), Options{})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(buf.String(), "[error opening dir]") {
		t.Errorf("missing annotation: %q", buf.String())
	}
}

func TestReport(t *testing.T) {
	tests := []struct {
		c        Counts
		dirsOnly bool
		want     string
	}{
		{Counts{3, 5}, false, "\n3 directories, 5 files\n"},
		{Counts{1, 1}, false, "\n1 directory, 1 file\n"},
		{Counts{0, 0}, false, "\n0 directories, 0 files\n"},
		{Counts{2, 9}, true, "\n2 directories\n"},
		{Counts{1, 0}, true, "\n1 directory\n"},
	}
	for _, tt := range tests {
		var buf bytes.Buffer
		if err := Report(&buf, tt.c, tt.dirsOnly); err != nil {
			t.Fatal(err)
		}
		if buf.String() != tt.want {
			t.Errorf("Report(%+v, %v) = %q, want %q", tt.c, tt.dirsOnly, buf.String(), tt.want)
		}
	}
}

func TestPrintSymlinkToDirCountedAsDir(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	got, c := render(t, root, Options{DirsOnly: true})
	want := "root\n├── link -> real\n└── real\n"
	if got != want || c != (Counts{Dirs: 3}) {
		t.Errorf("got:\n%s%+v\nwant:\n%s", got, c, want)
	}
}

func TestPrintRootCountedOnlyWhenListed(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".hidden"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, c := render(t, root, Options{}); c != (Counts{}) {
		t.Errorf("hidden only: counts = %+v, want zero", c)
	}
	if _, c := render(t, root, Options{All: true}); c != (Counts{Dirs: 1, Files: 1}) {
		t.Errorf("with -a: counts = %+v", c)
	}
}

func TestPrintFileArgument(t *testing.T) {
	f := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(f, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	c, err := Print(&buf, f, Options{})
	if err == nil || c != (Counts{Files: 1}) {
		t.Errorf("counts = %+v, err = %v", c, err)
	}
}

func TestPrintUnreadableSubdir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod does not restrict directory reads on Windows")
	}
	root := fixture(t)
	locked := filepath.Join(root, "dir2")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	if os.Geteuid() == 0 {
		t.Skip("root can read any directory")
	}

	var buf bytes.Buffer
	if _, err := Print(&buf, root, Options{}); err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(buf.String(), "dir2  [error opening dir]") {
		t.Errorf("missing annotation:\n%s", buf.String())
	}
}

func TestPrintStopsAfterWriteError(t *testing.T) {
	root := fixture(t)
	// First write is the root line; fail on the first entry after it.
	w := &failAfter{n: 1}
	_, err := Print(w, root, Options{})
	if err == nil {
		t.Fatal("expected an error")
	}
	if w.writes != 2 {
		t.Errorf("writes = %d, want 2 (stop at first failure)", w.writes)
	}
}

type failAfter struct{ n, writes int }

func (f *failAfter) Write(p []byte) (int, error) {
	f.writes++
	if f.writes > f.n {
		return 0, errors.New("closed pipe")
	}
	return len(p), nil
}

func TestPrintCombinedOptions(t *testing.T) {
	got, c := render(t, fixture(t), Options{DirsOnly: true, DirsFirst: true, MaxDepth: 1})
	want := "root\n├── dir1\n└── dir2\n"
	if got != want || c != (Counts{Dirs: 3}) {
		t.Errorf("got:\n%s%+v", got, c)
	}
}
