package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GyeongHoKim/build-your-own-coreutils/internal/filetime"
)

func TestTimestampCalibration(t *testing.T) {
	fx, err := newFixture(t.TempDir(), time.Local)
	if err != nil {
		t.Fatal(err)
	}
	if fx.skip != "" {
		t.Skipf("filesystem cannot support the fixture: %s", fx.skip)
	}
	t.Logf("resolution %s slack %s step %s", fx.resolution, fx.slack, fx.step)
	if fx.resolution < 0 || fx.resolution > 2*time.Second || fx.slack < 0 || fx.slack > time.Second {
		t.Fatalf("got resolution %s slack %s, want small non-negative values", fx.resolution, fx.slack)
	}

	want := filetime.Times{Access: seedAccess, Modify: seedModify}
	stored, err := fx.store(want)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Access.Equal(stored.Modify) {
		t.Fatalf("got equal stored times %+v, want distinct ones", stored)
	}
	if d := stored.Access.Sub(want.Access).Abs(); d > fx.resolution {
		t.Errorf("stored access differs by %s, more than resolution %s", d, fx.resolution)
	}

	r := &result{started: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), finished: time.Date(2026, 1, 1, 0, 0, 1, 0, time.UTC)}
	lo, hi := fx.window(r)
	if lo.After(r.started) || hi.Before(r.finished) {
		t.Errorf("window [%s, %s] does not contain the invocation [%s, %s]", lo, hi, r.started, r.finished)
	}
	if r.started.Sub(lo) > fx.resolution+fx.slack || hi.Sub(r.finished) > fx.resolution+fx.step {
		t.Errorf("window [%s, %s] is wider than the measured resolution", lo, hi)
	}
}

func TestFixtureSkipsWithoutDistinctTimes(t *testing.T) {
	fx := &fixture{}
	fx.checkStored(filetime.Times{Access: seedAccess, Modify: seedModify}, filetime.Times{Access: seedModify, Modify: seedModify})
	if fx.skip == "" {
		t.Fatal("want a skip reason when access time cannot be set")
	}
}

func TestWrongLearnerAccessIsFailure(t *testing.T) {
	fx := &fixture{}
	snap := snapshot{exists: true, times: filetime.Times{Access: seedModify, Modify: seedModify}}
	failure := fx.timesFailure(&result{}, &snap, exactly(seedAccess), exactly(seedModify))
	if failure == "" || !strings.Contains(failure, "access") {
		t.Fatalf("got failure %q, want an access time failure", failure)
	}
}

func TestCurrentTimeWant(t *testing.T) {
	fx := &fixture{}
	r := &result{started: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), finished: time.Date(2026, 1, 1, 0, 0, 1, 0, time.UTC)}
	inside := snapshot{exists: true, times: filetime.Times{Access: r.started, Modify: r.finished}}
	if failure := fx.timesFailure(r, &inside, current, current); failure != "" {
		t.Errorf("times within the invocation: got failure %q", failure)
	}
	before := snapshot{exists: true, times: filetime.Times{Access: r.started.Add(-time.Second), Modify: r.finished}}
	if failure := fx.timesFailure(r, &before, current, current); failure == "" {
		t.Error("access time before the invocation: want a failure")
	}
}

func TestLocalDateExpectations(t *testing.T) {
	fixed := time.FixedZone("UTC+9", 9*60*60)
	got, skip := localTime(fixed, 2026, time.June, 15, 12, 30, 0)
	if skip != "" || !got.Equal(time.Date(2026, 6, 15, 3, 30, 0, 0, time.UTC)) {
		t.Fatalf("got %s (skip %q), want 2026-06-15T03:30:00Z", got, skip)
	}

	if got, skip := localTime(fixed, 2000, time.January, 15, 10, 0, 30); skip != "" || !got.Equal(time.Date(2000, 1, 15, 1, 0, 30, 0, time.UTC)) {
		t.Fatalf("got %s (skip %q), want 2000-01-15T01:00:30Z", got, skip)
	}

	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("time zone database unavailable: %v", err)
	}
	for _, hour := range []int{1, 2, 3} {
		if _, skip := localTime(newYork, 2026, time.March, 8, hour, 30, 0); skip == "" {
			t.Errorf("hour %d near the March DST change: want a skip", hour)
		}
	}
	if _, skip := localTime(newYork, 2026, time.November, 1, 1, 30, 0); skip == "" {
		t.Error("ambiguous November time: want a skip")
	}
	if _, skip := localTime(newYork, 2026, time.June, 15, 12, 30, 0); skip != "" {
		t.Errorf("ordinary date: got skip %q", skip)
	}
}

func TestOmittedYears(t *testing.T) {
	tests := []struct {
		name              string
		started, finished time.Time
		want              []int
	}{
		{"same year", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 6, 1, 0, 0, 1, 0, time.UTC), []int{2026}},
		{"new year", time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC), time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), []int{2026, 2027}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := omittedYears(tt.started, tt.finished, time.UTC)
			if len(got) != len(tt.want) || got[0] != tt.want[0] || got[len(got)-1] != tt.want[len(tt.want)-1] {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
	east := time.FixedZone("UTC+9", 9*60*60)
	if got := omittedYears(time.Date(2026, 12, 31, 15, 0, 0, 0, time.UTC), time.Date(2026, 12, 31, 15, 0, 1, 0, time.UTC), east); got[0] != 2027 {
		t.Errorf("got %v, want the local year 2027", got)
	}
}

func readContentUpdatesAccess(t *testing.T, dir string) bool {
	t.Helper()
	probe := filepath.Join(dir, "probe")
	if err := os.WriteFile(probe, []byte("probe"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(probe, seedAccess, seedModify); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(probe); err != nil {
		t.Fatal(err)
	}
	after, err := filetime.Read(probe)
	if err != nil {
		t.Fatal(err)
	}
	return !after.Access.Equal(seedAccess)
}

func TestMetadataBeforeContent(t *testing.T) {
	dir := t.TempDir()
	if !readContentUpdatesAccess(t, dir) {
		t.Skip("reading content does not update access time on this filesystem")
	}
	path := filepath.Join(dir, "file")
	if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, seedAccess, seedModify); err != nil {
		t.Fatal(err)
	}

	snap, err := inspect(path)
	if err != nil {
		t.Fatal(err)
	}
	if !snap.times.Access.Equal(seedAccess) {
		t.Errorf("got access %s, want the pre-read value %s", snap.times.Access, seedAccess)
	}
	if snap.content != "content" {
		t.Errorf("got content %q, want %q", snap.content, "content")
	}
}

func TestInspectMissing(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "regular")
	if err := os.WriteFile(regular, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(dir, "missing"), filepath.Join(regular, "child")} {
		snap, err := inspect(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if snap.exists {
			t.Errorf("%s: want it to be reported absent", path)
		}
	}
}

func TestWindowAllowsClockStepAfterInvocation(t *testing.T) {
	fx := &fixture{step: 15 * time.Millisecond}
	r := &result{started: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), finished: time.Date(2026, 1, 1, 0, 0, 1, 0, time.UTC)}
	_, hi := fx.window(r)
	if precise := r.finished.Add(10 * time.Millisecond); precise.After(hi) {
		t.Fatalf("window ends at %s, want it to include a precise clock reading %s within one measured step", hi, precise)
	}
}

func TestSeededFixtureKeepsAccessTime(t *testing.T) {
	dir := t.TempDir()
	if !readContentUpdatesAccess(t, dir) {
		t.Skip("reading content does not update access time on this filesystem")
	}
	fx, err := newFixture(t.TempDir(), time.Local)
	if err != nil {
		t.Fatal(err)
	}
	b := newCaseBuilder(fx, dir)
	seeded := b.file("target")
	ref := b.reference()
	if b.err != nil {
		t.Fatal(b.err)
	}
	for name, want := range map[string]time.Time{"target": seeded.times.Access, refName: ref.Access} {
		got, err := filetime.Read(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if !got.Access.Equal(want) {
			t.Errorf("%s: access time %s after setup, want the captured %s", name, got.Access, want)
		}
	}
}
