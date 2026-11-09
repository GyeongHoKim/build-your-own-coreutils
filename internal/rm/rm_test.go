package rm

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

type removal struct {
	path string
	dir  bool
}

type failure struct {
	path string
	err  error
}

// record captures every callback made by Remove.
type record struct {
	answers   []bool
	questions []string
	removed   []removal
	failed    []failure
}

// collect returns a record and Callbacks that write to it. Confirm returns the
// scripted answers in order and "no" once they run out.
func collect(answers ...bool) (*record, Callbacks) {
	r := &record{answers: answers}
	cb := Callbacks{
		Confirm: func(question string) bool {
			r.questions = append(r.questions, question)
			if len(r.answers) == 0 {
				return false
			}
			a := r.answers[0]
			r.answers = r.answers[1:]
			return a
		},
		Removed: func(path string, dir bool) {
			r.removed = append(r.removed, removal{path, dir})
		},
		Failed: func(path string, err error) {
			r.failed = append(r.failed, failure{path, err})
		},
	}
	return r, cb
}

// inTempDir changes the working directory to a new temporary directory.
func inTempDir(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
}

func writeFile(t *testing.T, name string) {
	t.Helper()
	if err := os.WriteFile(name, nil, 0o666); err != nil {
		t.Fatal(err)
	}
}

func mkdir(t *testing.T, name string) {
	t.Helper()
	if err := os.Mkdir(name, 0o777); err != nil {
		t.Fatal(err)
	}
}

func symlink(t *testing.T, target, name string) {
	t.Helper()
	if err := os.Symlink(target, name); err != nil {
		t.Skipf("cannot create symbolic link: %v", err)
	}
}

func exists(t *testing.T, name string) bool {
	t.Helper()
	_, err := os.Lstat(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

func assertFailed(t *testing.T, r *record, path string, target error) {
	t.Helper()
	if len(r.failed) != 1 {
		t.Fatalf("failures = %v, want one for %q", r.failed, path)
	}
	f := r.failed[0]
	if f.path != path || !errors.Is(f.err, target) {
		t.Fatalf("failure = %q: %v, want %q: %v", f.path, f.err, path, target)
	}
	if _, ok := errors.AsType[*fs.PathError](f.err); ok {
		t.Errorf("failure error %v keeps its *fs.PathError wrapper", f.err)
	}
}

func TestRemoveFile(t *testing.T) {
	inTempDir(t)
	writeFile(t, "f")
	r, cb := collect()

	if !Remove([]string{"f"}, Options{}, cb) {
		t.Error("Remove reported failure")
	}
	if exists(t, "f") {
		t.Error("f still exists")
	}
	if want := []removal{{"f", false}}; !slices.Equal(r.removed, want) {
		t.Errorf("removed = %v, want %v", r.removed, want)
	}
	if len(r.failed) != 0 || len(r.questions) != 0 {
		t.Errorf("failures = %v, questions = %v, want none", r.failed, r.questions)
	}
}

func TestMissingOperand(t *testing.T) {
	inTempDir(t)
	// An empty pathname names nothing; it is not ".".
	for _, op := range []string{"missing", filepath.Join("nodir", "missing"), ""} {
		t.Run(op, func(t *testing.T) {
			r, cb := collect()
			if Remove([]string{op}, Options{}, cb) {
				t.Error("Remove reported success")
			}
			assertFailed(t, r, op, fs.ErrNotExist)

			r, cb = collect()
			if !Remove([]string{op}, Options{Force: true}, cb) {
				t.Error("Remove with Force reported failure")
			}
			if len(r.failed) != 0 || len(r.removed) != 0 || len(r.questions) != 0 {
				t.Errorf("Force made callbacks: %+v", r)
			}
		})
	}
}

func TestNilCallbacks(t *testing.T) {
	inTempDir(t)
	writeFile(t, "f")
	if !Remove([]string{"f", "missing"}, Options{Force: true, Interactive: true}, Callbacks{}) {
		t.Error("Remove reported failure")
	}
	if Remove([]string{"missing"}, Options{}, Callbacks{}) {
		t.Error("Remove reported success")
	}
	writeFile(t, "g")
	if !Remove([]string{"g"}, Options{Interactive: true}, Callbacks{}) {
		t.Error("declined removal reported failure")
	}
	if !exists(t, "g") {
		t.Error("nil Confirm did not answer no")
	}
}

func TestDirectoryWithoutRecursive(t *testing.T) {
	inTempDir(t)
	mkdir(t, "d")
	r, cb := collect()

	if Remove([]string{"d"}, Options{}, cb) {
		t.Error("Remove reported success")
	}
	assertFailed(t, r, "d", ErrIsDir)
	if !exists(t, "d") {
		t.Error("d was removed")
	}
}

func TestDirsRemovesEmptyDirectory(t *testing.T) {
	inTempDir(t)
	mkdir(t, "d")
	r, cb := collect()

	if !Remove([]string{"d"}, Options{Dirs: true}, cb) {
		t.Error("Remove reported failure")
	}
	if exists(t, "d") {
		t.Error("d still exists")
	}
	if want := []removal{{"d", true}}; !slices.Equal(r.removed, want) {
		t.Errorf("removed = %v, want %v", r.removed, want)
	}
}

func TestDirsTrailingSlash(t *testing.T) {
	inTempDir(t)
	mkdir(t, "d")
	r, cb := collect()

	if !Remove([]string{"d/"}, Options{Dirs: true}, cb) {
		t.Errorf("Remove reported failure: %v", r.failed)
	}
	if exists(t, "d") {
		t.Error("d still exists")
	}
	if want := []removal{{"d/", true}}; !slices.Equal(r.removed, want) {
		t.Errorf("removed = %v, want %v", r.removed, want)
	}
}

func TestDirsNonEmptyFails(t *testing.T) {
	inTempDir(t)
	mkdir(t, "d")
	writeFile(t, filepath.Join("d", "f"))
	r, cb := collect()

	if Remove([]string{"d"}, Options{Dirs: true}, cb) {
		t.Error("Remove reported success")
	}
	if len(r.failed) != 1 || r.failed[0].path != "d" {
		t.Fatalf("failures = %v, want one for d", r.failed)
	}
	if !exists(t, filepath.Join("d", "f")) {
		t.Error("d/f was removed")
	}
}

func TestDotOperands(t *testing.T) {
	inTempDir(t)
	mkdir(t, "a")
	writeFile(t, filepath.Join("a", "f"))
	mkdir(t, "e")

	// Dirs would remove the empty directory e if the check were missing.
	for _, op := range []string{".", "..", "a/.", "a/..", "a/./", "e/.", "e/./", "e/.."} {
		t.Run(op, func(t *testing.T) {
			r, cb := collect(true)
			if Remove([]string{op}, Options{Dirs: true}, cb) {
				t.Error("Remove reported success")
			}
			assertFailed(t, r, op, ErrDot)
			if len(r.questions) != 0 || len(r.removed) != 0 {
				t.Errorf("questions = %v, removed = %v, want none", r.questions, r.removed)
			}
		})
	}
	if !exists(t, filepath.Join("a", "f")) || !exists(t, "e") {
		t.Error("an entry was removed")
	}
}

func rootPath(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "windows" {
		return "/"
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.VolumeName(cwd) + `\`
}

// These tests never set Recursive or Force: a missing check must not be able
// to damage the system.
func TestRootOperand(t *testing.T) {
	root := rootPath(t)
	r, cb := collect(true)
	if Remove([]string{root}, Options{}, cb) {
		t.Error("Remove reported success")
	}
	assertFailed(t, r, root, ErrRoot)
	if len(r.questions) != 0 {
		t.Errorf("questions = %v, want none", r.questions)
	}

	t.Run("symlink with trailing slash", func(t *testing.T) {
		inTempDir(t)
		symlink(t, root, "rootlink")
		op := "rootlink" + string(filepath.Separator)
		r, cb := collect(true)
		if Remove([]string{op}, Options{}, cb) {
			t.Error("Remove reported success")
		}
		assertFailed(t, r, op, ErrRoot)
		if !exists(t, "rootlink") {
			t.Error("rootlink was removed")
		}
	})

	t.Run("symlink without trailing slash", func(t *testing.T) {
		inTempDir(t)
		symlink(t, root, "rootlink")
		r, cb := collect()
		if !Remove([]string{"rootlink"}, Options{}, cb) {
			t.Errorf("Remove reported failure: %v", r.failed)
		}
		if exists(t, "rootlink") {
			t.Error("rootlink still exists")
		}
	})
}

// isRoot decides by identity, so only links that lead to the root, followed
// by a trailing separator, count as the root.
func TestIsRoot(t *testing.T) {
	inTempDir(t)
	root := rootPath(t)
	mkdir(t, "dir")
	writeFile(t, "f")
	symlink(t, root, "rootlink")
	symlink(t, "dir", "dirlink")
	sep := string(filepath.Separator)
	tests := []struct {
		op   string
		want bool
	}{
		{root, true},
		{"rootlink" + sep, true},
		{"rootlink" + sep + sep, true},
		{"rootlink", false},
		{"dirlink" + sep, false},
		{"dirlink", false},
		{"dir" + sep, false},
		{"dir", false},
		{"f", false},
	}
	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			trimmed := trimSeparators(tt.op)
			got, err := isRoot(tt.op, trimmed, trimmed != tt.op)
			if err != nil || got != tt.want {
				t.Errorf("isRoot(%q) = %v, %v, want %v, nil", tt.op, got, err, tt.want)
			}
		})
	}

	t.Run("missing", func(t *testing.T) {
		got, err := isRoot("missing", "missing", false)
		if got || !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("isRoot(missing) = %v, %v, want false, ErrNotExist", got, err)
		}
	})
}

func TestInteractiveFile(t *testing.T) {
	inTempDir(t)
	writeFile(t, "f")
	want := []string{"remove regular empty file 'f'?"}

	r, cb := collect(false)
	if !Remove([]string{"f"}, Options{Interactive: true}, cb) {
		t.Error("declined removal reported failure")
	}
	if !exists(t, "f") {
		t.Error("f was removed after answer no")
	}
	if !slices.Equal(r.questions, want) {
		t.Errorf("questions = %q, want %q", r.questions, want)
	}
	if len(r.removed) != 0 {
		t.Errorf("removed = %v, want none", r.removed)
	}

	r, cb = collect(true)
	if !Remove([]string{"f"}, Options{Interactive: true}, cb) {
		t.Error("Remove reported failure")
	}
	if exists(t, "f") {
		t.Error("f still exists after answer yes")
	}
	if !slices.Equal(r.questions, want) {
		t.Errorf("questions = %q, want %q", r.questions, want)
	}
}

func TestQuestionKinds(t *testing.T) {
	inTempDir(t)
	if err := os.WriteFile("full", []byte("x"), 0o666); err != nil {
		t.Fatal(err)
	}
	mkdir(t, "d")
	ops := []string{"full", "d"}
	want := []string{"remove regular file 'full'?", "remove directory 'd'?"}
	if err := os.Symlink("full", "link"); err == nil {
		ops = append(ops, "link")
		want = append(want, "remove symbolic link 'link'?")
	}

	r, cb := collect()
	Remove(ops, Options{Interactive: true, Dirs: true}, cb)
	if !slices.Equal(r.questions, want) {
		t.Errorf("questions = %q, want %q", r.questions, want)
	}
}

func TestWriteProtectedPrompt(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("nothing is write-protected for the superuser")
	}
	// os.Chmod sets the read-only attribute on Windows.
	setup := func(t *testing.T) {
		t.Helper()
		inTempDir(t)
		writeFile(t, "f")
		if err := os.Chmod("f", 0o444); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("terminal", func(t *testing.T) {
		setup(t)
		r, cb := collect(false, true)
		Remove([]string{"f"}, Options{Terminal: true}, cb)
		want := []string{"remove write-protected regular empty file 'f'?"}
		if !slices.Equal(r.questions, want) {
			t.Errorf("questions = %q, want %q", r.questions, want)
		}
		if !exists(t, "f") {
			t.Fatal("f was removed after answer no")
		}
		if !Remove([]string{"f"}, Options{Terminal: true}, cb) {
			t.Errorf("Remove reported failure: %v", r.failed)
		}
		if exists(t, "f") {
			t.Error("f still exists after answer yes")
		}
	})

	for _, tt := range []struct {
		name string
		opts Options
	}{
		{"not a terminal", Options{}},
		{"force", Options{Terminal: true, Force: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setup(t)
			r, cb := collect()
			if !Remove([]string{"f"}, tt.opts, cb) {
				t.Errorf("Remove reported failure: %v", r.failed)
			}
			if len(r.questions) != 0 {
				t.Errorf("questions = %q, want none", r.questions)
			}
			if exists(t, "f") {
				t.Error("f still exists")
			}
		})
	}
}

func TestSymlinkOperandRemovesLink(t *testing.T) {
	inTempDir(t)
	mkdir(t, "dir")
	writeFile(t, filepath.Join("dir", "f"))
	symlink(t, "dir", "link")
	r, cb := collect()

	if !Remove([]string{"link"}, Options{}, cb) {
		t.Errorf("Remove reported failure: %v", r.failed)
	}
	if exists(t, "link") {
		t.Error("link still exists")
	}
	if !exists(t, filepath.Join("dir", "f")) {
		t.Error("the link target was changed")
	}
	if want := []removal{{"link", false}}; !slices.Equal(r.removed, want) {
		t.Errorf("removed = %v, want %v", r.removed, want)
	}
}

// "link/../victim" names victim in the parent of link's target, because ".."
// is resolved after following link. Cleaning the path lexically would remove
// ./victim instead.
func TestSymlinkDotDotOperand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows resolves .. lexically before following links")
	}
	inTempDir(t)
	mkdir(t, "elsewhere")
	mkdir(t, filepath.Join("elsewhere", "sub"))
	writeFile(t, "victim")
	writeFile(t, filepath.Join("elsewhere", "victim"))
	symlink(t, filepath.Join("elsewhere", "sub"), "link")
	r, cb := collect()

	if !Remove([]string{"link/../victim"}, Options{}, cb) {
		t.Errorf("Remove reported failure: %v", r.failed)
	}
	if exists(t, filepath.Join("elsewhere", "victim")) {
		t.Error("elsewhere/victim still exists")
	}
	if !exists(t, "victim") {
		t.Error("./victim was removed")
	}
}

func TestSplitOperand(t *testing.T) {
	sep := string(os.PathSeparator)
	tests := []struct{ in, dir, name string }{
		{"f", ".", "f"},
		{"d" + sep + "f", "d" + sep, "f"},
		{"link" + sep + ".." + sep + "f", "link" + sep + ".." + sep, "f"},
		{"d" + sep + sep + "f", "d" + sep + sep, "f"},
		{sep + "f", sep, "f"},
	}
	for _, tt := range tests {
		if dir, name := splitOperand(tt.in); dir != tt.dir || name != tt.name {
			t.Errorf("splitOperand(%q) = %q, %q; want %q, %q", tt.in, dir, name, tt.dir, tt.name)
		}
	}
}

func TestIsRootHandle(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{rootPath(t), true},
		{t.TempDir(), false},
	}
	for _, tt := range tests {
		dir, err := os.OpenRoot(tt.path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := isRootHandle(dir)
		_ = dir.Close()
		if err != nil || got != tt.want {
			t.Errorf("isRootHandle(%q) = %v, %v; want %v", tt.path, got, err, tt.want)
		}
	}
}

// If "link/" is redirected to the root after the operand was checked, the
// directory actually opened must still be refused. A stand-in directory
// plays the root, so a missing check cannot damage the system.
func TestLinkRedirectedToRootRefused(t *testing.T) {
	inTempDir(t)
	mkdir(t, "dir")
	mkdir(t, "fakeroot")
	writeFile(t, filepath.Join("fakeroot", "f"))
	symlink(t, "dir", "link")
	fake, err := os.Stat("fakeroot")
	if err != nil {
		t.Fatal(err)
	}
	oldCheck := openedIsRoot
	openedIsRoot = func(dir *os.Root) (bool, error) {
		st, err := dir.Stat(".")
		return err == nil && os.SameFile(st, fake), err
	}
	beforeOpen = func(name string) {
		if name != "link/" {
			return
		}
		if err := os.Remove("link"); err != nil {
			t.Error(err)
		}
		if err := os.Symlink("fakeroot", "link"); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { openedIsRoot, beforeOpen = oldCheck, nil })
	r, cb := collect()

	if Remove([]string{"link/"}, Options{Recursive: true}, cb) {
		t.Error("Remove reported success")
	}
	assertFailed(t, r, "link/", ErrRoot)
	if !exists(t, filepath.Join("fakeroot", "f")) {
		t.Error("the redirected directory was emptied")
	}
}

func TestSymlinkToDirectoryTrailingSlash(t *testing.T) {
	inTempDir(t)
	mkdir(t, "dir")
	symlink(t, "dir", "link")
	r, cb := collect()

	if Remove([]string{"link/"}, Options{}, cb) {
		t.Error("Remove reported success")
	}
	assertFailed(t, r, "link/", ErrIsDir)
	if !exists(t, "link") || !exists(t, "dir") {
		t.Error("an entry was removed")
	}
}

func TestTrailingSlashOnFile(t *testing.T) {
	inTempDir(t)
	writeFile(t, "f")
	r, cb := collect()

	if Remove([]string{"f/"}, Options{Force: true}, cb) {
		t.Error("Remove reported success")
	}
	assertFailed(t, r, "f/", ErrNotDir)
	if !exists(t, "f") {
		t.Error("f was removed")
	}
}

func TestOperandsContinueAfterFailure(t *testing.T) {
	inTempDir(t)
	writeFile(t, "f")
	r, cb := collect()

	if Remove([]string{"missing", ".", "f"}, Options{}, cb) {
		t.Error("Remove reported success")
	}
	if len(r.failed) != 2 {
		t.Errorf("failures = %v, want two", r.failed)
	}
	if exists(t, "f") {
		t.Error("f was not removed")
	}
}
