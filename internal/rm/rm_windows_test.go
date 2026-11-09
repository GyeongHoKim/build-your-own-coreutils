package rm

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
)

// errSharingViolation is ERROR_SHARING_VIOLATION, which package syscall
// does not define.
const errSharingViolation syscall.Errno = 32

// openHandle opens path with CreateFile, sharing it as share allows, and
// keeps the handle open until the test ends. Call it after the temporary
// directory is created: cleanups run in reverse order, so the handle is
// closed before the directory is removed.
func openHandle(t *testing.T, path string, share, flags uint32) {
	t.Helper()
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ, share, nil, syscall.OPEN_EXISTING, flags, 0)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	t.Cleanup(func() {
		if err := syscall.CloseHandle(h); err != nil {
			t.Error(err)
		}
	})
}

// setReadOnly sets FILE_ATTRIBUTE_READONLY on path. The attribute is
// cleared when the test ends, so the temporary directory can be removed.
func setReadOnly(t *testing.T, path string) {
	t.Helper()
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	attrs, err := syscall.GetFileAttributes(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.SetFileAttributes(p, attrs|syscall.FILE_ATTRIBUTE_READONLY); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// The entry is usually gone by now.
		if attrs, err := syscall.GetFileAttributes(p); err == nil {
			_ = syscall.SetFileAttributes(p, attrs&^syscall.FILE_ATTRIBUTE_READONLY)
		}
	})
}

const shareAll = syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE | syscall.FILE_SHARE_DELETE

// os.Root removes entries with POSIX semantics, so an entry whose open
// handles all permit deletion disappears at once and its parent can be
// removed. This needs NTFS.
func TestOpenWithShareDeleteIsRemoved(t *testing.T) {
	inTempDir(t)
	mkdir(t, "d")
	f := filepath.Join("d", "f")
	writeFile(t, f)
	openHandle(t, f, shareAll, syscall.FILE_ATTRIBUTE_NORMAL)
	r, cb := collect()

	if !Remove([]string{"d"}, Options{Recursive: true}, cb) {
		t.Errorf("Remove reported failure: %v", r.failed)
	}
	if exists(t, "d") {
		t.Error("d still exists")
	}
}

func TestOpenDirectoryHandleIsRemoved(t *testing.T) {
	inTempDir(t)
	mkdir(t, "d")
	sub := filepath.Join("d", "sub")
	mkdir(t, sub)
	openHandle(t, sub, shareAll, syscall.FILE_FLAG_BACKUP_SEMANTICS)
	r, cb := collect()

	if !Remove([]string{"d"}, Options{Recursive: true}, cb) {
		t.Errorf("Remove reported failure: %v", r.failed)
	}
	if exists(t, "d") {
		t.Error("d still exists")
	}
}

func TestLockedFileFailsOthersRemoved(t *testing.T) {
	inTempDir(t)
	mkdir(t, "d")
	locked := filepath.Join("d", "locked")
	writeFile(t, locked)
	writeFile(t, "other")
	openHandle(t, locked, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, syscall.FILE_ATTRIBUTE_NORMAL)
	r, cb := collect()

	if Remove([]string{"d", "other"}, Options{Recursive: true}, cb) {
		t.Error("Remove reported success")
	}
	if exists(t, "other") {
		t.Error("other still exists")
	}
	if !exists(t, locked) {
		t.Errorf("%s was removed", locked)
	}
	var paths []string
	for _, f := range r.failed {
		paths = append(paths, f.path)
	}
	if want := []string{`d\locked`, "d"}; !slices.Equal(paths, want) {
		t.Fatalf("failures = %v, want %q", r.failed, want)
	}
	if err := r.failed[0].err; !errors.Is(err, errSharingViolation) {
		t.Errorf("error for %s = %v, want a sharing violation", locked, err)
	}
}

func TestReadOnlyFile(t *testing.T) {
	t.Run("force", func(t *testing.T) {
		inTempDir(t)
		writeFile(t, "f")
		setReadOnly(t, "f")
		r, cb := collect()

		if !Remove([]string{"f"}, Options{Force: true, Terminal: true}, cb) {
			t.Errorf("Remove reported failure: %v", r.failed)
		}
		if len(r.questions) != 0 {
			t.Errorf("questions = %q, want none", r.questions)
		}
		if exists(t, "f") {
			t.Error("f still exists")
		}
	})

	t.Run("terminal answer no", func(t *testing.T) {
		inTempDir(t)
		writeFile(t, "f")
		setReadOnly(t, "f")
		r, cb := collect(false)

		if !Remove([]string{"f"}, Options{Terminal: true}, cb) {
			t.Errorf("declined removal reported failure: %v", r.failed)
		}
		if want := []string{"remove write-protected regular empty file 'f'?"}; !slices.Equal(r.questions, want) {
			t.Errorf("questions = %q, want %q", r.questions, want)
		}
		if !exists(t, "f") {
			t.Error("f was removed after answer no")
		}
	})
}

// Windows ignores the read-only attribute on directories, so it does not
// make one write-protected.
func TestReadOnlyDirectoryNoPrompt(t *testing.T) {
	inTempDir(t)
	mkdir(t, "d")
	writeFile(t, filepath.Join("d", "f"))
	setReadOnly(t, "d")
	r, cb := collect()

	if !Remove([]string{"d"}, Options{Recursive: true, Terminal: true}, cb) {
		t.Errorf("Remove reported failure: %v", r.failed)
	}
	if len(r.questions) != 0 {
		t.Errorf("questions = %q, want none", r.questions)
	}
	if exists(t, "d") {
		t.Error("d still exists")
	}
}

// mklinkJunction creates a directory junction at link pointing to the
// absolute path target. It skips the test if it cannot, except in CI, where
// junction coverage must not disappear silently. Only test setup runs cmd;
// rm itself never does.
func mklinkJunction(t *testing.T, link, target string) {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err == nil {
		return
	}
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		t.Fatalf("cannot create junction: %v: %s", err, out)
	}
	t.Skipf("cannot create junction: %v: %s", err, out)
}

func TestJunctionNotFollowed(t *testing.T) {
	inTempDir(t)
	mkdir(t, "outside")
	keep := filepath.Join("outside", "keep")
	writeFile(t, keep)
	mkdir(t, "tree")
	target, err := filepath.Abs("outside")
	if err != nil {
		t.Fatal(err)
	}
	mklinkJunction(t, filepath.Join("tree", "j"), target)
	r, cb := collect()

	if !Remove([]string{"tree"}, Options{Recursive: true}, cb) {
		t.Errorf("Remove reported failure: %v", r.failed)
	}
	if exists(t, "tree") {
		t.Error("tree still exists")
	}
	if !exists(t, keep) {
		t.Errorf("%s was removed through the junction", keep)
	}
	if want := []removal{{`tree\j`, false}, {"tree", true}}; !slices.Equal(r.removed, want) {
		t.Errorf("removed = %v, want %v", r.removed, want)
	}
}

func TestLongPath(t *testing.T) {
	inTempDir(t)
	component := strings.Repeat("n", 40)
	top := "top"
	deep := top
	for range 8 {
		deep = filepath.Join(deep, component)
	}
	if len(deep) <= 300 {
		t.Fatalf("relative path has %d characters, want more than 300", len(deep))
	}
	if err := os.MkdirAll(deep, 0o777); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(deep, "f")
	writeFile(t, file)

	t.Run("absolute operand", func(t *testing.T) {
		abs, err := filepath.Abs(file)
		if err != nil {
			t.Fatal(err)
		}
		r, cb := collect()
		if !Remove([]string{abs}, Options{}, cb) {
			t.Errorf("Remove reported failure: %v", r.failed)
		}
		if exists(t, abs) {
			t.Errorf("%s still exists", abs)
		}
	})

	t.Run("tree", func(t *testing.T) {
		writeFile(t, file)
		r, cb := collect()
		if !Remove([]string{top}, Options{Recursive: true}, cb) {
			t.Errorf("Remove reported failure: %v", r.failed)
		}
		if exists(t, top) {
			t.Errorf("%s still exists", top)
		}
		if len(r.removed) == 0 || r.removed[0].path != file {
			t.Errorf("first removal = %v, want %s", r.removed, file)
		}
	})
}

// This test never sets Recursive or Force: a missing check must not be able
// to damage the system.
func TestDriveRootRefused(t *testing.T) {
	vol := filepath.VolumeName(t.TempDir())
	for _, root := range []string{vol + `\`, vol + `/`} {
		t.Run(root, func(t *testing.T) {
			r, cb := collect(true)
			if Remove([]string{root}, Options{}, cb) {
				t.Error("Remove reported success")
			}
			assertFailed(t, r, root, ErrRoot)
			if len(r.questions) != 0 {
				t.Errorf("questions = %q, want none", r.questions)
			}
		})
	}
}

// Lstat reports a junction as irregular rather than a directory, so
// filepath.EvalSymlinks does not resolve it, yet "j\" leads through it.
// This test never sets Recursive or Force: a missing check must not be able
// to damage the system.
func TestJunctionToDriveRootRefused(t *testing.T) {
	inTempDir(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	mklinkJunction(t, "j", filepath.VolumeName(cwd)+`\`)
	// Remove the junction itself before the temporary directory is cleaned
	// up, so cleanup never has to deal with it.
	t.Cleanup(func() {
		if err := os.Remove(filepath.Join(cwd, "j")); err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Error(err)
		}
	})
	for _, op := range []string{`j\`, `j/`, `j\\`} {
		t.Run(op, func(t *testing.T) {
			r, cb := collect(true)
			if Remove([]string{op}, Options{}, cb) {
				t.Error("Remove reported success")
			}
			assertFailed(t, r, op, ErrRoot)
			if len(r.questions) != 0 || len(r.removed) != 0 {
				t.Errorf("questions = %q, removed = %v, want none", r.questions, r.removed)
			}
			if !exists(t, "j") {
				t.Error("j was removed")
			}
		})
	}
}

func TestJunctionToDirectoryTrailingSlash(t *testing.T) {
	inTempDir(t)
	mkdir(t, "dir")
	target, err := filepath.Abs("dir")
	if err != nil {
		t.Fatal(err)
	}
	mklinkJunction(t, "j", target)
	r, cb := collect()

	if Remove([]string{`j\`}, Options{}, cb) {
		t.Error("Remove reported success")
	}
	assertFailed(t, r, `j\`, ErrIsDir)
	if !exists(t, "j") || !exists(t, "dir") {
		t.Error("an entry was removed")
	}
}

func TestIsVolumeRootPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{`\\?\C:\`, true},
		{`\\?\C:\dir`, false},
		{`\\?\C:\dir\`, false},
		{`\\?\Volume{0b3c9c6a-1d2e-4f00-8a6b-000000000000}\`, true},
		{`\\?\Volume{0b3c9c6a-1d2e-4f00-8a6b-000000000000}\dir`, false},
		{`\\?\UNC\server\share\`, true},
		{`\\?\UNC\server\share`, true},
		{`\\?\UNC\server\share\dir`, false},
	}
	for _, tt := range tests {
		if got := isVolumeRootPath(tt.path); got != tt.want {
			t.Errorf("isVolumeRootPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
