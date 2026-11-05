package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/GyeongHoKim/build-your-own-coreutils/internal/filetime"
)

const fakeTouchEnv = "GRADE_TEST_FAKE_TOUCH"

const (
	touchCorrect          = "correct"
	touchCreateOnly       = "create-only"
	touchTruncate         = "truncate"
	touchUpdateBoth       = "update-both"
	touchCopyModifyToBoth = "copy-mtime-to-both"
	touchIgnoreNoCreate   = "ignore-no-create"
	touchAcceptBadInput   = "accept-bad-input"
	touchStopOnFirstError = "stop-on-first-error"
	touchUnexpectedStdout = "unexpected-stdout"
	touchHang             = "hang"
	touchGNUStyle         = "gnu-style"
	touchPOSIXStyle       = "posix-style"
)

const touchHangDuration = 1500 * time.Millisecond

type fakeTouch struct {
	mode     string
	access   bool
	modify   bool
	noCreate bool
	ref      string
	stamp    string
	refSet   bool
	stampSet bool
	files    []string
}

func runFakeTouch(mode string, args []string) int {
	ft := &fakeTouch{mode: mode}
	usageStatus := 2
	if mode == touchGNUStyle || mode == touchPOSIXStyle {
		usageStatus = 1
	}
	if err := ft.parse(args); err != nil {
		if mode == touchAcceptBadInput {
			return 0
		}
		ft.warn("%v", err)
		return usageStatus
	}
	if mode == touchHang && slices.Contains(ft.files, createCaseTarget) {
		time.Sleep(touchHangDuration)
	}

	now := time.Now()
	times := filetime.Times{Access: now, Modify: now}
	switch {
	case ft.stampSet:
		t, err := ft.parseTime(ft.stamp, now)
		if err != nil {
			if mode == touchAcceptBadInput {
				return 0
			}
			ft.warn("invalid date %q", ft.stamp)
			return usageStatus
		}
		times = filetime.Times{Access: t, Modify: t}
	case ft.refSet:
		var err error
		if times, err = filetime.Read(ft.ref); err != nil {
			ft.warn("%s: %v", ft.ref, err)
			return 1
		}
		if mode == touchCopyModifyToBoth {
			times.Access = times.Modify
		}
	}
	if mode == touchUpdateBoth {
		ft.access, ft.modify = false, false
	}
	if mode == touchIgnoreNoCreate {
		ft.noCreate = false
	}

	status := 0
	for _, path := range ft.files {
		if path == "-" && mode == touchGNUStyle {
			continue
		}
		if err := ft.update(path, times); err != nil {
			ft.warn("%s: %v", path, err)
			status = 1
			if mode == touchStopOnFirstError {
				return status
			}
		}
	}
	if status == 0 && mode == touchUnexpectedStdout {
		fmt.Println("done")
	}
	return status
}

func (ft *fakeTouch) warn(format string, a ...any) {
	prefix := "touch: "
	if ft.mode == touchPOSIXStyle {
		prefix = filepath.Base(os.Args[0]) + ": "
	}
	fmt.Fprintf(os.Stderr, prefix+format+"\n", a...)
}

func (ft *fakeTouch) parse(args []string) error {
	gnu := ft.mode == touchGNUStyle
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			ft.files = append(ft.files, args[i+1:]...)
			break
		}
		if len(arg) < 2 || arg[0] != '-' || (len(ft.files) > 0 && !gnu) {
			ft.files = append(ft.files, arg)
			continue
		}
		flags := arg[1:]
		if !gnu && len(flags) > 1 {
			return fmt.Errorf("unknown option %s", arg)
		}
		for j := 0; j < len(flags); j++ {
			switch flags[j] {
			case 'a':
				ft.access = true
			case 'm':
				ft.modify = true
			case 'c':
				ft.noCreate = true
			case 'r', 't':
				value := flags[j+1:]
				if value == "" {
					if i+1 >= len(args) {
						return fmt.Errorf("option -%c needs a value", flags[j])
					}
					i++
					value = args[i]
				}
				if flags[j] == 'r' {
					ft.ref, ft.refSet = value, true
				} else {
					ft.stamp, ft.stampSet = value, true
				}
				j = len(flags)
			default:
				return fmt.Errorf("unknown option %s", arg)
			}
		}
	}
	switch {
	case ft.refSet && ft.stampSet:
		return errors.New("cannot use both -r and -t")
	case ft.refSet && ft.ref == "":
		return errors.New("empty reference")
	case len(ft.files) == 0:
		return errors.New("missing file operand")
	}
	return nil
}

func (ft *fakeTouch) parseTime(value string, now time.Time) (time.Time, error) {
	main, secs, hasSecs := strings.Cut(value, ".")
	if strings.ContainsFunc(main+secs, func(r rune) bool { return r < '0' || r > '9' }) || (hasSecs && len(secs) != 2) {
		return time.Time{}, errors.New("syntax")
	}
	n := func(s string) int { v, _ := strconv.Atoi(s); return v }
	year := now.Year()
	switch len(main) {
	case 8:
	case 10:
		year = 2000 + n(main[:2])
		if year >= 2069 {
			year -= 100
		}
		main = main[2:]
	case 12:
		year, main = n(main[:4]), main[4:]
	default:
		return time.Time{}, errors.New("length")
	}
	month, day, hour, minute, second := n(main[0:2]), n(main[2:4]), n(main[4:6]), n(main[6:8]), 0
	if hasSecs {
		second = n(secs)
	}
	maxSecond := 59
	if ft.mode == touchGNUStyle {
		maxSecond = 60
	}
	if second > maxSecond {
		return time.Time{}, errors.New("seconds")
	}
	t := time.Date(year, time.Month(month), day, hour, minute, min(second, 59), 0, now.Location())
	if second == 60 {
		t = t.Add(time.Second)
	}
	if t.Year() != year || int(t.Month()) != month || t.Day() != day || t.Hour() != hour || t.Minute() != minute {
		if second != 60 {
			return time.Time{}, errors.New("calendar")
		}
	}
	if ft.mode == touchPOSIXStyle && (t.Before(time.Unix(0, 0)) || !t.Before(time.Date(2038, 1, 1, 0, 0, 0, 0, time.UTC))) {
		return time.Time{}, errors.New("out of range")
	}
	return t, nil
}

func (ft *fakeTouch) update(path string, times filetime.Times) error {
	access, modify := times.Access, times.Modify
	if ft.access != ft.modify {
		if !ft.access {
			access = time.Time{}
		}
		if !ft.modify {
			modify = time.Time{}
		}
	}
	if _, err := os.Stat(path); err == nil {
		if ft.mode == touchCreateOnly {
			return nil
		}
		if ft.mode == touchTruncate {
			if err := os.Truncate(path, 0); err != nil {
				return err
			}
		}
		return os.Chtimes(path, access, modify)
	}
	if ft.noCreate {
		parent, err := os.Stat(filepath.Dir(path))
		if err == nil && !parent.IsDir() {
			return errors.New("not a directory")
		}
		return nil
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0o666)
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if ft.mode == touchCreateOnly {
		return nil
	}
	return os.Chtimes(path, access, modify)
}

func gradeFake(t *testing.T, mode, executable string, timeout time.Duration) (status int, stdout, stderr string) {
	t.Helper()
	t.Setenv(fakeTouchEnv, mode)
	var out, errOut bytes.Buffer
	status = gradeTouchTimeout(executable, timeout, &out, &errOut)
	return status, out.String(), errOut.String()
}

func TestGradeTouchCorrect(t *testing.T) {
	status, stdout, stderr := gradeFake(t, touchCorrect, testExecutable(t), caseTimeout)
	if status != 0 || stderr != "" {
		t.Fatalf("got status %d, stderr %q\n%s", status, stderr, stdout)
	}
	for _, c := range touchCases(nil) {
		if !strings.Contains(stdout, "PASS "+c.name+"\n") && !strings.Contains(stdout, "SKIP "+c.name+":") {
			t.Errorf("case %s did not pass:\n%s", c.name, stdout)
		}
	}
}

func TestGradeTouchReferenceStyles(t *testing.T) {
	for _, mode := range []string{touchGNUStyle, touchPOSIXStyle} {
		t.Run(mode, func(t *testing.T) {
			executable := filepath.Join(t.TempDir(), "my-touch"+filepath.Ext(os.Args[0]))
			copyExecutable(t, executable)
			status, stdout, stderr := gradeFake(t, mode, executable, caseTimeout)
			if status != 0 || stderr != "" {
				t.Fatalf("got status %d, stderr %q\n%s", status, stderr, stdout)
			}
		})
	}
}

func TestGradeTouchDefects(t *testing.T) {
	tests := []struct {
		mode    string
		timeout time.Duration
		failing []string
	}{
		{touchCreateOnly, caseTimeout, []string{"existing", "select-a"}},
		{touchTruncate, caseTimeout, []string{"existing"}},
		{touchUpdateBoth, caseTimeout, []string{"select-a", "select-m", "reference-a"}},
		{touchCopyModifyToBoth, caseTimeout, []string{"reference-both", "reference-a"}},
		{touchIgnoreNoCreate, caseTimeout, []string{"no-create-missing", "no-create-parent"}},
		{touchAcceptBadInput, caseTimeout, []string{"invalid-option", "invalid-no-files", "invalid-date"}},
		{touchStopOnFirstError, caseTimeout, []string{"error-continue"}},
		{touchUnexpectedStdout, caseTimeout, []string{"create", "existing"}},
		{touchHang, time.Second, []string{"create"}},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			status, stdout, stderr := gradeFake(t, tt.mode, testExecutable(t), tt.timeout)
			if status != 1 {
				t.Fatalf("got status %d, want 1; stderr %q\n%s", status, stderr, stdout)
			}
			for _, name := range tt.failing {
				if !strings.Contains(stdout, "FAIL "+name+": ") {
					t.Errorf("want FAIL %s:\n%s", name, stdout)
				}
			}
			if tt.mode == touchHang {
				if !strings.Contains(stdout, "FAIL create: ") || !strings.Contains(stdout, "timed out") {
					t.Errorf("want the create case to time out:\n%s", stdout)
				}
				if !strings.Contains(stdout, "PASS existing\n") || !strings.Contains(stdout, "PASS link-dangling-no-create\n") &&
					!strings.Contains(stdout, "SKIP link-dangling-no-create:") {
					t.Errorf("want later cases to keep running:\n%s", stdout)
				}
			}
		})
	}
}

func TestGradeTouchUsesCaseTimeout(t *testing.T) {
	if caseTimeout != 10*time.Second {
		t.Fatalf("got caseTimeout %s, want 10s", caseTimeout)
	}
	if touchHangDuration >= caseTimeout {
		t.Fatal("the hang helper must finish within the production timeout")
	}
	t.Setenv(fakeTouchEnv, touchHang)
	var stdout, stderr bytes.Buffer
	if status := gradeTouch(testExecutable(t), &stdout, &stderr); status != 0 {
		t.Fatalf("got status %d, want the brief hang to pass under caseTimeout\n%s%s", status, &stdout, &stderr)
	}
}

func TestGradeTouchInfrastructure(t *testing.T) {
	t.Run("startup failure", func(t *testing.T) {
		executable := filepath.Join(t.TempDir(), "not-executable.exe")
		if err := os.WriteFile(executable, []byte("not a program"), 0o600); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		if status := gradeTouchTimeout(executable, caseTimeout, &stdout, &stderr); status != 2 || stderr.Len() == 0 {
			t.Fatalf("got status %d, stderr %q; want 2 with a message", status, &stderr)
		}
	})

	t.Run("setup failure", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing")
		t.Setenv("TMPDIR", missing)
		t.Setenv("TMP", missing)
		t.Setenv("TEMP", missing)
		t.Setenv(fakeTouchEnv, touchCorrect)
		var stdout, stderr bytes.Buffer
		if status := gradeTouchTimeout(testExecutable(t), caseTimeout, &stdout, &stderr); status != 2 || stderr.Len() == 0 {
			t.Fatalf("got status %d, stderr %q; want 2 with a message", status, &stderr)
		}
	})
}

func TestTouchSummary(t *testing.T) {
	tests := []struct {
		name       string
		outcomes   []caseOutcome
		graderErr  error
		wantStatus int
		wantLines  []string
	}{
		{
			name: "all pass",
			outcomes: []caseOutcome{
				{name: "a", tier: tierCore, kind: outcomePass},
				{name: "b", tier: tierExtended, kind: outcomePass},
			},
			wantStatus: 0,
			wantLines:  []string{"core: 1/1 passed", "extended: 1/1 passed", "2/2 passed"},
		},
		{
			name: "failure and skip",
			outcomes: []caseOutcome{
				{name: "a", tier: tierCore, kind: outcomeFail},
				{name: "b", tier: tierCore, kind: outcomePass},
				{name: "c", tier: tierExtended, kind: outcomeSkip},
			},
			wantStatus: 1,
			wantLines:  []string{"core: 1/2 passed", "extended: 0/0 passed", "1/2 passed", "1 skipped (coverage reduced)"},
		},
		{
			name:       "all skipped",
			outcomes:   []caseOutcome{{name: "a", tier: tierCore, kind: outcomeSkip}},
			wantStatus: 2,
		},
		{
			name:       "grader error wins",
			outcomes:   []caseOutcome{{name: "a", tier: tierCore, kind: outcomeFail}},
			graderErr:  errors.New("cannot remove /tmp/x"),
			wantStatus: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := summarize(&stdout, &stderr, tt.outcomes, tt.graderErr); got != tt.wantStatus {
				t.Fatalf("got status %d, want %d\n%s%s", got, tt.wantStatus, &stdout, &stderr)
			}
			lines := strings.Split(stdout.String(), "\n")
			last := -1
			for _, want := range tt.wantLines {
				i := slices.Index(lines, want)
				if i <= last {
					t.Errorf("want line %q after the previous summary line:\n%s", want, &stdout)
				}
				last = i
			}
			if tt.wantStatus == 2 && stderr.Len() == 0 {
				t.Error("want a message on stderr")
			}
		})
	}
}

func TestTouchCaseOrderAndScope(t *testing.T) {
	cases := touchCases(nil)
	seenExtended := false
	names := map[string]bool{}
	for _, c := range cases {
		if names[c.name] {
			t.Errorf("duplicate case %s", c.name)
		}
		names[c.name] = true
		switch c.tier {
		case tierCore:
			if seenExtended {
				t.Errorf("core case %s follows an extended case", c.name)
			}
		case tierExtended:
			seenExtended = true
		default:
			t.Errorf("case %s has tier %q", c.name, c.tier)
		}
		if strings.Contains(c.name, "help") || strings.Contains(c.name, "68") || strings.Contains(c.name, "69") {
			t.Errorf("case %s is outside the graded scope", c.name)
		}
	}

	fx, err := newFixture(t.TempDir(), time.Local)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range touchCases(fx) {
		inv, _, err := c.prepare(t.TempDir())
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		for i, arg := range inv.args {
			if arg == "--help" || arg == "-h" {
				t.Errorf("%s invokes help", c.name)
			}
			if arg == "-t" && i+1 < len(inv.args) && !gradedDate(inv.args[i+1]) {
				t.Errorf("%s uses -t %s outside 1970-2037 or the 00/26 two-digit years", c.name, inv.args[i+1])
			}
		}
	}
	for _, seed := range []time.Time{seedAccess, seedModify, refAccess, refModify} {
		if seed.Year() < 1970 || seed.Year() > 2037 {
			t.Errorf("fixture time %s is outside 1970-2037", seed)
		}
	}
}

func gradedDate(value string) bool {
	main, _, _ := strings.Cut(value, ".")
	if strings.ContainsFunc(main, func(r rune) bool { return r < '0' || r > '9' }) {
		return true
	}
	switch len(main) {
	case 10:
		return main[:2] == "00" || main[:2] == "26"
	case 12:
		year, err := strconv.Atoi(main[:4])
		return err == nil && year >= 1970 && year <= 2037
	default:
		return true
	}
}

func TestTouchDiagnosticNames(t *testing.T) {
	path := filepath.Join("missing", "child")
	for _, stderr := range []string{
		"touch: cannot touch '" + path + "': no such file or directory\n",
		"my-touch.exe: " + path + ": not found\n",
		"my-touch: " + path + "\n",
		path + ": no such file or directory\n",
	} {
		if failure := pathDiagnosticFailure(stderr, path); failure != "" {
			t.Errorf("stderr %q: got failure %q", stderr, failure)
		}
	}
	if failure := pathDiagnosticFailure("touch: failed\n", path); failure == "" {
		t.Error("want a failure when the path is missing from the diagnostic")
	}
}

func TestTouchHelperContracts(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "my-touch"+filepath.Ext(os.Args[0]))
	copyExecutable(t, executable)

	call := func(t *testing.T, mode string, args ...string) *result {
		t.Helper()
		t.Setenv(fakeTouchEnv, mode)
		work := t.TempDir()
		r, err := invoke(executable, work, args, caseTimeout)
		if err != nil {
			t.Fatal(err)
		}
		return &r
	}
	tests := []struct {
		name  string
		mode  string
		args  []string
		check func(r *result) string
	}{
		{"gnu accepts combined options", touchGNUStyle, []string{"-am", "file"}, wantStatus(0)},
		{"strict rejects combined options", touchCorrect, []string{"-am", "file"}, wantStatus(2)},
		{"gnu usage status is 1", touchGNUStyle, []string{"-z", "file"}, wantStatus(1)},
		{"gnu accepts seconds 60", touchGNUStyle, []string{"-t", "202606151230.60", "file"}, wantStatus(0)},
		{"strict rejects seconds 60", touchCorrect, []string{"-t", "202606151230.60", "file"}, wantStatus(2)},
		{"gnu permutes options", touchGNUStyle, []string{"file", "-z"}, wantStatus(1)},
		{"strict treats late options as files", touchCorrect, []string{"file", "-z"}, wantStatus(0)},
		{"gnu treats - as stdout", touchGNUStyle, []string{"-"}, wantStatus(0)},
		{"posix rejects help", touchPOSIXStyle, []string{"--help"}, wantStatus(1)},
		{"posix rejects pre-Epoch", touchPOSIXStyle, []string{"-t", "196906151230", "file"}, wantStatus(1)},
		{"posix rejects 2038", touchPOSIXStyle, []string{"-t", "203801021200", "file"}, wantStatus(1)},
		{"posix accepts 2037", touchPOSIXStyle, []string{"-t", "203712312359", "file"}, wantStatus(0)},
		{"posix uses its basename", touchPOSIXStyle, []string{"-z", "file"}, func(r *result) string {
			if !strings.HasPrefix(r.stderr, "my-touch") {
				return fmt.Sprintf("stderr %q does not start with the executable name", r.stderr)
			}
			return ""
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if failure := tt.check(call(t, tt.mode, tt.args...)); failure != "" {
				t.Error(failure)
			}
		})
	}

	t.Run("gnu does not create -", func(t *testing.T) {
		t.Setenv(fakeTouchEnv, touchGNUStyle)
		work := t.TempDir()
		if _, err := invoke(executable, work, []string{"-"}, caseTimeout); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(work, "-")); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("got %v, want no file named -", err)
		}
	})
}

func wantStatus(status int) func(r *result) string {
	return func(r *result) string {
		if r.status != status {
			return fmt.Sprintf("got status %d (stderr %q), want %d", r.status, r.stderr, status)
		}
		return ""
	}
}

func TestTouchCasesExplainThemselves(t *testing.T) {
	for _, c := range touchCases(nil) {
		if c.summary == "" {
			t.Errorf("case %s has no summary", c.name)
		}
	}
	status, stdout, _ := gradeFake(t, touchTruncate, testExecutable(t), caseTimeout)
	if status != 1 {
		t.Fatalf("got status %d, want 1", status)
	}
	for _, c := range touchCases(nil) {
		if c.name == "existing" && !strings.Contains(stdout, "FAIL existing: "+c.summary+"; args ") {
			t.Errorf("FAIL line does not start with the case summary:\n%s", stdout)
		}
	}
}
