package rm

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

// makeTree creates dirs directories under root, each holding files empty
// files, and returns the number of files created.
func makeTree(tb testing.TB, root string, dirs, files int) int {
	tb.Helper()
	for i := range dirs {
		dir := filepath.Join(root, fmt.Sprintf("d%03d", i))
		if err := os.MkdirAll(dir, 0o777); err != nil {
			tb.Fatal(err)
		}
		for j := range files {
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%03d", j)), nil, 0o666); err != nil {
				tb.Fatal(err)
			}
		}
	}
	return dirs * files
}

func TestJobsRemoveConcurrently(t *testing.T) {
	inTempDir(t)
	mkdir(t, "d")
	writeFile(t, filepath.Join("d", "a"))
	writeFile(t, filepath.Join("d", "b"))
	// Each removal waits until both are in progress, which happens only
	// when siblings are removed concurrently.
	var inFlight atomic.Int32
	var timedOut atomic.Bool
	both := make(chan struct{})
	beforeRemove = func(string) {
		if inFlight.Add(1) == 2 {
			close(both)
		}
		select {
		case <-both:
		case <-time.After(2 * time.Second):
			timedOut.Store(true)
		}
	}
	t.Cleanup(func() { beforeRemove = nil })
	r, cb := collect()

	if !Remove([]string{"d"}, Options{Recursive: true, Jobs: 2}, cb) {
		t.Errorf("Remove reported failure: %v", r.failed)
	}
	if timedOut.Load() {
		t.Error("siblings were not removed concurrently with Jobs 2")
	}
}

func TestCallbacksSerialized(t *testing.T) {
	inTempDir(t)
	want := makeTree(t, "t", 20, 100)
	var inCallback bool
	files := 0 // deliberately not atomic: the race detector checks serialization
	cb := Callbacks{
		Removed: func(_ string, dir bool) {
			if inCallback {
				t.Error("Removed called while another callback is active")
			}
			inCallback = true
			if !dir {
				files++
			}
			inCallback = false
		},
		Failed: func(path string, err error) { t.Errorf("failed %s: %v", path, err) },
	}

	if !Remove([]string{"t"}, Options{Recursive: true, Jobs: 8}, cb) {
		t.Error("Remove reported failure")
	}
	if files != want {
		t.Errorf("removed %d files, want %d", files, want)
	}
	if exists(t, "t") {
		t.Error("t still exists")
	}
}

func TestPromptBlocksOtherOutput(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("nothing is write-protected for the superuser")
	}
	inTempDir(t)
	mkdir(t, "d")
	for i := range 200 {
		writeFile(t, filepath.Join("d", fmt.Sprintf("f%03d", i)))
	}
	protected := filepath.Join("d", "f100")
	// os.Chmod sets the read-only attribute on Windows.
	if err := os.Chmod(protected, 0o444); err != nil {
		t.Fatal(err)
	}
	var inPrompt bool
	var questions []string
	removed := 0
	cb := Callbacks{
		Confirm: func(q string) bool {
			questions = append(questions, q)
			inPrompt = true
			time.Sleep(50 * time.Millisecond)
			inPrompt = false
			return true
		},
		Removed: func(path string, _ bool) {
			if inPrompt {
				t.Errorf("Removed(%s) called while a prompt is open", path)
			}
			removed++
		},
		Failed: func(path string, err error) { t.Errorf("failed %s: %v", path, err) },
	}

	if !Remove([]string{"d"}, Options{Recursive: true, Terminal: true, Jobs: 8}, cb) {
		t.Error("Remove reported failure")
	}
	want := []string{"remove write-protected regular empty file '" + protected + "'?"}
	if !slices.Equal(questions, want) {
		t.Errorf("questions = %q, want %q", questions, want)
	}
	if removed != 201 {
		t.Errorf("removed %d entries, want 201", removed)
	}
}

func TestInteractiveIsSequential(t *testing.T) {
	inTempDir(t)
	mkdir(t, "d")
	for _, name := range []string{"c", "a", "b"} {
		writeFile(t, filepath.Join("d", name))
	}
	var active, maxActive atomic.Int32
	enter := func() func() {
		n := active.Add(1)
		if n > maxActive.Load() {
			maxActive.Store(n)
		}
		time.Sleep(time.Millisecond) // widen the window for overlapping calls
		return func() { active.Add(-1) }
	}
	r, inner := collect(true, true, true, true, true)
	cb := Callbacks{
		Confirm: func(q string) bool { defer enter()(); return inner.Confirm(q) },
		Removed: func(p string, dir bool) { defer enter()(); inner.Removed(p, dir) },
		Failed:  func(p string, err error) { defer enter()(); inner.Failed(p, err) },
	}

	if !Remove([]string{"d"}, Options{Recursive: true, Interactive: true, Jobs: 8}, cb) {
		t.Errorf("Remove reported failure: %v", r.failed)
	}
	j := filepath.Join
	want := []string{
		"descend into directory 'd'?",
		"remove regular empty file '" + j("d", "a") + "'?",
		"remove regular empty file '" + j("d", "b") + "'?",
		"remove regular empty file '" + j("d", "c") + "'?",
		"remove directory 'd'?",
	}
	if !slices.Equal(r.questions, want) {
		t.Errorf("questions = %q, want %q", r.questions, want)
	}
	wantRemoved := []removal{{j("d", "a"), false}, {j("d", "b"), false}, {j("d", "c"), false}, {"d", true}}
	if !slices.Equal(r.removed, wantRemoved) {
		t.Errorf("removed = %v, want %v", r.removed, wantRemoved)
	}
	if m := maxActive.Load(); m != 1 {
		t.Errorf("%d callbacks were active at once, want 1", m)
	}
}

func TestParentAfterChildren(t *testing.T) {
	inTempDir(t)
	for i := range 8 {
		makeTree(t, filepath.Join("t", fmt.Sprintf("s%d", i)), 8, 10)
	}
	r, cb := collect()

	if !Remove([]string{"t"}, Options{Recursive: true, Jobs: 8}, cb) {
		t.Errorf("Remove reported failure: %v", r.failed)
	}
	if want := 1 + 8 + 8*8 + 8*8*10; len(r.removed) != want {
		t.Fatalf("removed %d entries, want %d", len(r.removed), want)
	}
	seen := make(map[string]bool)
	for _, e := range r.removed {
		if seen[e.path] {
			t.Errorf("%s reported twice", e.path)
		}
		seen[e.path] = true
		// Every ancestor must still be present when e is reported.
		for p := filepath.Dir(e.path); p != "."; p = filepath.Dir(p) {
			if seen[p] {
				t.Errorf("%s reported after its directory %s", e.path, p)
			}
		}
	}
	if last := r.removed[len(r.removed)-1]; last != (removal{"t", true}) {
		t.Errorf("last removal = %v, want t", last)
	}
}
