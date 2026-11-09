package rm

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// skipUnlessUnixUser skips tests that rely on Unix permission bits denying
// the current user.
func skipUnlessUnixUser(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("directory permission bits do not restrict removal on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("permission bits do not restrict the superuser")
	}
}

// chmod changes the mode of name and restores a removable mode when the
// test ends, so that the temporary directory can be cleaned up.
func chmod(t *testing.T, name string, mode os.FileMode) {
	t.Helper()
	abs, err := filepath.Abs(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(abs, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(abs, 0o755) })
}

func indexOf(removed []removal, path string) int {
	return slices.IndexFunc(removed, func(r removal) bool { return r.path == path })
}

func TestRecursiveRemovesTree(t *testing.T) {
	inTempDir(t)
	j := filepath.Join
	mkdir(t, "t")
	writeFile(t, j("t", "f"))
	mkdir(t, j("t", "a"))
	writeFile(t, j("t", "a", "g"))
	mkdir(t, j("t", "a", "b"))
	writeFile(t, j("t", "a", "b", "h"))
	mkdir(t, j("t", "a", "empty"))
	mkdir(t, j("t", "e"))
	r, cb := collect()

	if !Remove([]string{"t"}, Options{Recursive: true}, cb) {
		t.Errorf("Remove reported failure: %v", r.failed)
	}
	if exists(t, "t") {
		t.Error("t still exists")
	}
	want := []removal{
		{j("t", "a", "b", "h"), false},
		{j("t", "a", "b"), true},
		{j("t", "a", "empty"), true},
		{j("t", "a", "g"), false},
		{j("t", "a"), true},
		{j("t", "e"), true},
		{j("t", "f"), false},
		{"t", true},
	}
	if !slices.Equal(r.removed, want) {
		t.Errorf("removed = %v, want %v", r.removed, want)
	}
	// Directories come after their contents whatever the order of siblings.
	for _, dir := range r.removed {
		if !dir.dir {
			continue
		}
		for _, other := range r.removed {
			if strings.HasPrefix(other.path, dir.path+string(filepath.Separator)) &&
				indexOf(r.removed, other.path) > indexOf(r.removed, dir.path) {
				t.Errorf("%s reported after its directory %s", other.path, dir.path)
			}
		}
	}
	if len(r.failed) != 0 || len(r.questions) != 0 {
		t.Errorf("failures = %v, questions = %v, want none", r.failed, r.questions)
	}
}

func TestRecursiveTrailingSlashKeepsSpelling(t *testing.T) {
	inTempDir(t)
	mkdir(t, "d")
	writeFile(t, filepath.Join("d", "f"))
	r, cb := collect()

	if !Remove([]string{"d/"}, Options{Recursive: true}, cb) {
		t.Errorf("Remove reported failure: %v", r.failed)
	}
	if want := []removal{{"d/f", false}, {"d/", true}}; !slices.Equal(r.removed, want) {
		t.Errorf("removed = %v, want %v", r.removed, want)
	}
}

func TestRecursiveDoesNotFollowSymlink(t *testing.T) {
	inTempDir(t)
	mkdir(t, "outside")
	writeFile(t, filepath.Join("outside", "keep"))
	mkdir(t, "tree")
	symlink(t, filepath.Join("..", "outside"), filepath.Join("tree", "link"))
	r, cb := collect()

	if !Remove([]string{"tree"}, Options{Recursive: true}, cb) {
		t.Errorf("Remove reported failure: %v", r.failed)
	}
	if exists(t, "tree") {
		t.Error("tree still exists")
	}
	if !exists(t, filepath.Join("outside", "keep")) {
		t.Error("outside/keep was removed through the link")
	}
	want := []removal{{filepath.Join("tree", "link"), false}, {"tree", true}}
	if !slices.Equal(r.removed, want) {
		t.Errorf("removed = %v, want %v", r.removed, want)
	}
}

func TestRecursiveTrailingSlashSymlink(t *testing.T) {
	inTempDir(t)
	mkdir(t, "target")
	writeFile(t, filepath.Join("target", "a"))
	symlink(t, "target", "link")
	r, cb := collect()

	if Remove([]string{"link/"}, Options{Recursive: true}, cb) {
		t.Error("Remove reported success")
	}
	if exists(t, filepath.Join("target", "a")) {
		t.Error("target/a still exists")
	}
	if !exists(t, "target") || !exists(t, "link") {
		t.Error("target or link was removed")
	}
	assertFailed(t, r, "link/", ErrNotDir)
	if want := []removal{{"link/a", false}}; !slices.Equal(r.removed, want) {
		t.Errorf("removed = %v, want %v", r.removed, want)
	}
}

func TestDirsTrailingSlashSymlink(t *testing.T) {
	inTempDir(t)
	mkdir(t, "target")
	symlink(t, "target", "link")
	r, cb := collect()

	if Remove([]string{"link/"}, Options{Dirs: true}, cb) {
		t.Error("Remove reported success")
	}
	assertFailed(t, r, "link/", ErrNotDir)
	if !exists(t, "target") || !exists(t, "link") {
		t.Error("target or link was removed")
	}
}

func TestInteractiveDeclineChildFailsParent(t *testing.T) {
	inTempDir(t)
	mkdir(t, "d")
	writeFile(t, filepath.Join("d", "f"))
	r, cb := collect(true, false, true)

	if Remove([]string{"d"}, Options{Recursive: true, Interactive: true}, cb) {
		t.Error("Remove reported success")
	}
	want := []string{
		"descend into directory 'd'?",
		"remove regular empty file '" + filepath.Join("d", "f") + "'?",
		"remove directory 'd'?",
	}
	if !slices.Equal(r.questions, want) {
		t.Errorf("questions = %q, want %q", r.questions, want)
	}
	if !exists(t, filepath.Join("d", "f")) {
		t.Error("d/f was removed after answer no")
	}
	if len(r.failed) != 1 || r.failed[0].path != "d" {
		t.Fatalf("failures = %v, want one for d", r.failed)
	}
	if msg := strings.ToLower(r.failed[0].err.Error()); !strings.Contains(msg, "not empty") {
		t.Errorf("failure error = %v, want directory not empty", r.failed[0].err)
	}
}

func TestInteractiveDeclineDescend(t *testing.T) {
	inTempDir(t)
	mkdir(t, "d")
	writeFile(t, filepath.Join("d", "f"))
	r, cb := collect(false)

	if !Remove([]string{"d"}, Options{Recursive: true, Interactive: true}, cb) {
		t.Errorf("Remove reported failure: %v", r.failed)
	}
	if want := []string{"descend into directory 'd'?"}; !slices.Equal(r.questions, want) {
		t.Errorf("questions = %q, want %q", r.questions, want)
	}
	if !exists(t, filepath.Join("d", "f")) {
		t.Error("d/f was removed")
	}
	if len(r.removed) != 0 || len(r.failed) != 0 {
		t.Errorf("removed = %v, failures = %v, want none", r.removed, r.failed)
	}
}

func TestEmptyDirSkipsDescendPrompt(t *testing.T) {
	inTempDir(t)
	mkdir(t, "d")
	r, cb := collect(true)

	if !Remove([]string{"d"}, Options{Recursive: true, Interactive: true}, cb) {
		t.Errorf("Remove reported failure: %v", r.failed)
	}
	if want := []string{"remove directory 'd'?"}; !slices.Equal(r.questions, want) {
		t.Errorf("questions = %q, want %q", r.questions, want)
	}
	if exists(t, "d") {
		t.Error("d still exists")
	}
}

func TestWriteProtectedDirPromptUnix(t *testing.T) {
	skipUnlessUnixUser(t)
	inTempDir(t)
	mkdir(t, "d")
	writeFile(t, filepath.Join("d", "f"))
	chmod(t, "d", 0o555)
	r, cb := collect(false)

	if !Remove([]string{"d"}, Options{Recursive: true, Terminal: true}, cb) {
		t.Errorf("Remove reported failure: %v", r.failed)
	}
	if want := []string{"descend into write-protected directory 'd'?"}; !slices.Equal(r.questions, want) {
		t.Errorf("questions = %q, want %q", r.questions, want)
	}
	if !exists(t, filepath.Join("d", "f")) {
		t.Error("d/f was removed")
	}
}

func TestDeepTree(t *testing.T) {
	inTempDir(t)
	const depth = 1000
	name := strings.Repeat("n", 10)
	// Build the tree through handles: its path is longer than PATH_MAX.
	r, err := os.OpenRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	for range depth {
		if err := r.Mkdir(name, 0o777); err != nil {
			t.Fatal(err)
		}
		child, err := r.OpenRoot(name)
		if err != nil {
			t.Fatal(err)
		}
		_ = r.Close()
		r = child
	}
	_ = r.Close()
	rec, cb := collect()

	if !Remove([]string{name}, Options{Recursive: true}, cb) {
		t.Fatalf("Remove reported failure: %d failures, first %v", len(rec.failed), rec.failed[:min(1, len(rec.failed))])
	}
	if exists(t, name) {
		t.Error("tree still exists")
	}
	if len(rec.removed) != depth {
		t.Fatalf("removed %d entries, want %d", len(rec.removed), depth)
	}
	if deepest := rec.removed[0].path; len(deepest) <= 10000 {
		t.Errorf("deepest path has %d characters, want more than 10000", len(deepest))
	}
}

func TestUnremovableEntryContinues(t *testing.T) {
	skipUnlessUnixUser(t)
	inTempDir(t)
	j := filepath.Join
	mkdir(t, "d")
	mkdir(t, j("d", "locked"))
	writeFile(t, j("d", "locked", "x"))
	writeFile(t, j("d", "other"))
	chmod(t, j("d", "locked"), 0o555)
	r, cb := collect()

	if Remove([]string{"d"}, Options{Recursive: true}, cb) {
		t.Error("Remove reported success")
	}
	if exists(t, j("d", "other")) {
		t.Error("d/other was not removed")
	}
	var paths []string
	for _, f := range r.failed {
		paths = append(paths, f.path)
	}
	if want := []string{j("d", "locked", "x"), j("d", "locked"), "d"}; !slices.Equal(paths, want) {
		t.Errorf("failures = %v, want %v", r.failed, want)
	}
}

func TestUnreadableDirectory(t *testing.T) {
	skipUnlessUnixUser(t)
	inTempDir(t)
	mkdir(t, "full")
	writeFile(t, filepath.Join("full", "f"))
	chmod(t, "full", 0o333)
	mkdir(t, "empty")
	chmod(t, "empty", 0o333)
	r, cb := collect()

	if Remove([]string{"full", "empty"}, Options{Recursive: true}, cb) {
		t.Error("Remove reported success")
	}
	assertFailed(t, r, "full", os.ErrPermission)
	if exists(t, "empty") {
		t.Error("the empty unreadable directory was not removed")
	}
}

func TestDirectoryReplacedBySymlink(t *testing.T) {
	inTempDir(t)
	mkdir(t, "victim")
	writeFile(t, filepath.Join("victim", "keep"))
	mkdir(t, "x")
	writeFile(t, filepath.Join("x", "f"))
	symlink(t, "victim", "probe") // skips where links cannot be made
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Swap x for a link to victim after Lstat has seen a directory.
	beforeOpen = func(name string) {
		if name != "x" {
			return
		}
		x := filepath.Join(cwd, "x")
		if err := os.RemoveAll(x); err != nil {
			t.Error(err)
		}
		if err := os.Symlink("victim", x); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { beforeOpen = nil })
	r, cb := collect()

	if Remove([]string{"x"}, Options{Recursive: true}, cb) {
		t.Error("Remove reported success")
	}
	assertFailed(t, r, "x", errReplaced)
	if !exists(t, filepath.Join("victim", "keep")) {
		t.Error("victim/keep was removed through the swapped link")
	}
	if len(r.removed) != 0 {
		t.Errorf("removed = %v, want none", r.removed)
	}
}

func TestForceIgnoresVanished(t *testing.T) {
	setup := func(t *testing.T) {
		t.Helper()
		inTempDir(t)
		mkdir(t, "d")
		writeFile(t, filepath.Join("d", "a"))
		writeFile(t, filepath.Join("d", "b"))
		dir, err := filepath.Abs("d")
		if err != nil {
			t.Fatal(err)
		}
		beforeRemove = func(name string) {
			if name == "a" {
				_ = os.Remove(filepath.Join(dir, name))
			}
		}
		t.Cleanup(func() { beforeRemove = nil })
	}

	t.Run("force", func(t *testing.T) {
		setup(t)
		r, cb := collect()
		if !Remove([]string{"d"}, Options{Recursive: true, Force: true}, cb) {
			t.Errorf("Remove reported failure: %v", r.failed)
		}
		if len(r.failed) != 0 {
			t.Errorf("failures = %v, want none", r.failed)
		}
		if exists(t, "d") {
			t.Error("d still exists")
		}
	})

	t.Run("no force", func(t *testing.T) {
		setup(t)
		r, cb := collect()
		if Remove([]string{"d"}, Options{Recursive: true}, cb) {
			t.Error("Remove reported success")
		}
		assertFailed(t, r, filepath.Join("d", "a"), os.ErrNotExist)
	})
}
