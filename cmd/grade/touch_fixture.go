package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/GyeongHoKim/build-your-own-coreutils/internal/filetime"
)

const (
	clockProbeLimit    = 30 * time.Millisecond
	timestampLayout    = "2006-01-02T15:04:05.000000000-07:00 MST"
	unambiguousMargin  = 2 * time.Hour
	resolutionProbeSec = 2
)

var (
	seedAccess = time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	seedModify = time.Date(2003, 4, 5, 6, 7, 8, 0, time.UTC)
)

type fixture struct {
	probeDir   string
	loc        *time.Location
	resolution time.Duration
	slack      time.Duration
	step       time.Duration
	skip       string
	probes     int
}

func newFixture(probeDir string, loc *time.Location) (*fixture, error) {
	fx := &fixture{probeDir: probeDir, loc: loc}
	requested := filetime.Times{
		Access: time.Date(2020, 1, 1, 0, 0, 0, 123456789, time.UTC),
		Modify: time.Date(2020, 1, 2, 0, 0, 0, 987654321, time.UTC),
	}
	stored, err := fx.store(requested)
	if err != nil {
		return nil, err
	}
	fx.resolution = max(stored.Access.Sub(requested.Access).Abs(), stored.Modify.Sub(requested.Modify).Abs())
	fx.checkStored(requested, stored)

	seeds, err := fx.store(filetime.Times{Access: seedAccess, Modify: seedModify})
	if err != nil {
		return nil, err
	}
	fx.checkStored(filetime.Times{Access: seedAccess, Modify: seedModify}, seeds)

	lag, step, err := fx.measureClock()
	if err != nil {
		return nil, err
	}
	fx.slack, fx.step = max(lag, step), step
	return fx, nil
}

func (fx *fixture) checkStored(requested, stored filetime.Times) {
	switch {
	case fx.skip != "":
	case stored.Access.Equal(stored.Modify):
		fx.skip = "the filesystem cannot store distinct access and modification times"
	case stored.Access.Sub(requested.Access).Abs() > resolutionProbeSec*time.Second:
		fx.skip = "the filesystem does not keep an explicitly set access time"
	case stored.Modify.Sub(requested.Modify).Abs() > resolutionProbeSec*time.Second:
		fx.skip = "the filesystem does not keep an explicitly set modification time"
	}
}

func (fx *fixture) probePath() string {
	fx.probes++
	return filepath.Join(fx.probeDir, fmt.Sprintf("probe-%d", fx.probes))
}

func (fx *fixture) store(want filetime.Times) (filetime.Times, error) {
	path := fx.probePath()
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		return filetime.Times{}, fmt.Errorf("timestamp probe: %w", err)
	}
	if err := os.Chtimes(path, want.Access, want.Modify); err != nil {
		return filetime.Times{}, fmt.Errorf("timestamp probe: %w", err)
	}
	stored, err := filetime.Read(path)
	if err != nil {
		return filetime.Times{}, fmt.Errorf("timestamp probe: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return filetime.Times{}, fmt.Errorf("timestamp probe: %w", err)
	}
	return stored, nil
}

func (fx *fixture) measureClock() (lag, step time.Duration, err error) {
	dir := filepath.Join(fx.probeDir, "clock")
	if err := os.Mkdir(dir, 0o700); err != nil {
		return 0, 0, fmt.Errorf("clock probe: %w", err)
	}
	type sample struct {
		path   string
		before time.Time
	}
	var samples []sample
	for deadline := time.Now().Add(clockProbeLimit); time.Now().Before(deadline); {
		s := sample{path: filepath.Join(dir, strconv.Itoa(len(samples))), before: time.Now()}
		if err := os.WriteFile(s.path, nil, 0o600); err != nil {
			return 0, 0, fmt.Errorf("clock probe: %w", err)
		}
		samples = append(samples, s)
	}

	var last time.Time
	for _, s := range samples {
		times, err := filetime.Read(s.path)
		if err != nil {
			return 0, 0, fmt.Errorf("clock probe: %w", err)
		}
		lag = max(lag, s.before.Sub(times.Access), s.before.Sub(times.Modify))
		if !last.IsZero() {
			step = max(step, times.Modify.Sub(last))
		}
		last = times.Modify
	}
	if err := os.RemoveAll(dir); err != nil {
		return 0, 0, fmt.Errorf("clock probe: %w", err)
	}
	return lag, step, nil
}

func (fx *fixture) window(r *result) (lo, hi time.Time) {
	return r.started.Add(-fx.resolution - fx.slack), r.finished.Add(fx.resolution + fx.step)
}

func (fx *fixture) format(t time.Time) string {
	loc := fx.loc
	if loc == nil {
		loc = time.Local
	}
	return t.In(loc).Format(timestampLayout)
}

func localTime(loc *time.Location, year int, month time.Month, day, hour, minute, second int) (t time.Time, skip string) {
	t = time.Date(year, month, day, hour, minute, second, 0, loc)
	if t.Year() != year || t.Month() != month || t.Day() != day || t.Hour() != hour || t.Minute() != minute || t.Second() != second {
		return time.Time{}, fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d does not exist in %s", year, month, day, hour, minute, second, loc)
	}
	_, offset := t.Zone()
	for _, d := range []time.Duration{-unambiguousMargin, unambiguousMargin} {
		if _, near := t.Add(d).Zone(); near != offset {
			return time.Time{}, fmt.Sprintf("%s is near a UTC offset change in %s", t.Format(time.DateTime), loc)
		}
	}
	return t, ""
}

func omittedYears(started, finished time.Time, loc *time.Location) []int {
	first, last := started.In(loc).Year(), finished.In(loc).Year()
	if first == last {
		return []int{first}
	}
	return []int{first, last}
}

type snapshot struct {
	exists  bool
	dir     bool
	regular bool
	size    int64
	times   filetime.Times
	content string
}

func inspect(path string) (snapshot, error) {
	s, err := inspectMetadata(path)
	if err != nil || !s.regular {
		return s, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return snapshot{}, fmt.Errorf("inspect %s: %w", path, err)
	}
	s.content = string(content)
	return s, nil
}

func inspectMetadata(path string) (snapshot, error) {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return snapshot{}, nil
	}
	if err != nil {
		return snapshot{}, fmt.Errorf("inspect %s: %w", path, err)
	}
	times, err := filetime.Read(path)
	if err != nil {
		return snapshot{}, fmt.Errorf("inspect %s: %w", path, err)
	}
	return snapshot{exists: true, dir: info.IsDir(), regular: info.Mode().IsRegular(), size: info.Size(), times: times}, nil
}

type timeWant struct {
	current bool
	fixed   []time.Time
}

func exactly(times ...time.Time) timeWant { return timeWant{fixed: times} }

var current = timeWant{current: true}

func (fx *fixture) timeFailure(r *result, field string, got time.Time, want timeWant) string {
	if want.current {
		lo, hi := fx.window(r)
		if got.Before(lo) || got.After(hi) {
			return fmt.Sprintf("%s time %s, want the current time between %s and %s", field, fx.format(got), fx.format(lo), fx.format(hi))
		}
		return ""
	}
	if slices.ContainsFunc(want.fixed, got.Equal) {
		return ""
	}
	wants := make([]string, len(want.fixed))
	for i, t := range want.fixed {
		wants[i] = fx.format(t)
	}
	return fmt.Sprintf("%s time %s, want %s", field, fx.format(got), strings.Join(wants, " or "))
}

func (fx *fixture) timesFailure(r *result, s *snapshot, access, modify timeWant) string {
	if failure := fx.timeFailure(r, "access", s.times.Access, access); failure != "" {
		return failure
	}
	return fx.timeFailure(r, "modification", s.times.Modify, modify)
}
