package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/GyeongHoKim/build-your-own-coreutils/internal/filetime"
)

const (
	tierCore     = "core"
	tierExtended = "extended"

	createCaseTarget = "create-target"
	fileContent      = "content"
	refName          = "ref"
)

var (
	refAccess = time.Date(2011, 3, 4, 5, 6, 7, 0, time.UTC)
	refModify = time.Date(2012, 8, 9, 10, 11, 12, 0, time.UTC)
)

type touchCheck func(r *result) (failure string, err error)

type touchInvocation struct {
	args      []string
	wantError bool
	check     touchCheck
}

type touchCase struct {
	name    string
	tier    string
	summary string
	prepare func(dir string) (touchInvocation, string, error)
}

type outcomeKind int

const (
	outcomePass outcomeKind = iota
	outcomeFail
	outcomeSkip
)

type caseOutcome struct {
	name    string
	tier    string
	kind    outcomeKind
	message string
}

func gradeTouch(executable string, stdout, stderr io.Writer) int {
	return gradeTouchTimeout(executable, caseTimeout, stdout, stderr)
}

func gradeTouchTimeout(executable string, timeout time.Duration, stdout, stderr io.Writer) int {
	probeDir, err := os.MkdirTemp("", "touch-grade-probe-")
	if err != nil {
		fmt.Fprintf(stderr, "cannot prepare fixtures: %v\n", err)
		return 2
	}
	out := &reportWriter{w: stdout}
	outcomes, graderErr := runTouchCases(executable, probeDir, timeout, out)
	if err := os.RemoveAll(probeDir); err != nil {
		graderErr = errors.Join(graderErr, fmt.Errorf("cannot remove %s: %w", probeDir, err))
	}
	status := summarize(out, stderr, outcomes, graderErr)
	if out.err != nil {
		fmt.Fprintf(stderr, "cannot write results: %v\n", out.err)
		return 2
	}
	return status
}

func runTouchCases(executable, probeDir string, timeout time.Duration, out io.Writer) ([]caseOutcome, error) {
	fx, err := newFixture(probeDir, time.Local)
	if err != nil {
		return nil, err
	}
	var outcomes []caseOutcome
	for _, c := range touchCases(fx) {
		o := caseOutcome{name: c.name, tier: c.tier, kind: outcomeSkip, message: fx.skip}
		if fx.skip == "" {
			if o, err = runTouchCase(c, executable, timeout); err != nil {
				return outcomes, fmt.Errorf("case %s: %w", c.name, err)
			}
		}
		outcomes = append(outcomes, o)
		switch o.kind {
		case outcomePass:
			fmt.Fprintf(out, "PASS %s\n", o.name)
		case outcomeFail:
			fmt.Fprintf(out, "FAIL %s: %s\n", o.name, o.message)
		case outcomeSkip:
			fmt.Fprintf(out, "SKIP %s: %s\n", o.name, o.message)
		}
	}
	return outcomes, nil
}

func runTouchCase(c touchCase, executable string, timeout time.Duration) (o caseOutcome, err error) {
	dir, err := os.MkdirTemp("", "touch-grade-")
	if err != nil {
		return o, fmt.Errorf("create fixture directory: %w", err)
	}
	defer func() {
		if rmErr := os.RemoveAll(dir); rmErr != nil {
			err = errors.Join(err, fmt.Errorf("cannot remove %s: %w", dir, rmErr))
		}
	}()

	o = caseOutcome{name: c.name, tier: c.tier}
	inv, skip, err := c.prepare(dir)
	if err != nil {
		return o, fmt.Errorf("prepare fixture: %w", err)
	}
	if skip != "" {
		o.kind, o.message = outcomeSkip, skip
		return o, nil
	}
	r, err := invoke(executable, dir, inv.args, timeout)
	if err != nil {
		return o, fmt.Errorf("cannot execute: %w", err)
	}
	failure, err := inv.judge(&r, timeout)
	if err != nil {
		return o, err
	}
	if failure != "" {
		o.kind, o.message = outcomeFail, fmt.Sprintf("%s; args %q: %s", c.summary, inv.args, failure)
	}
	return o, nil
}

func (inv touchInvocation) judge(r *result, timeout time.Duration) (string, error) {
	switch {
	case r.timedOut:
		return fmt.Sprintf("timed out after %s", timeout), nil
	case inv.wantError && (r.status == 0 || r.stdout != "" || r.stderr == ""):
		return fmt.Sprintf("want non-zero status, empty stdout and a message on stderr; got status %d, stdout %q, stderr %q",
			r.status, r.stdout, r.stderr), nil
	case !inv.wantError && (r.status != 0 || r.stdout != "" || r.stderr != ""):
		return fmt.Sprintf("want status 0 and no output; got status %d, stdout %q, stderr %q", r.status, r.stdout, r.stderr), nil
	}
	return inv.check(r)
}

func summarize(stdout, stderr io.Writer, outcomes []caseOutcome, graderErr error) int {
	passed := map[string]int{}
	executed := map[string]int{}
	skipped := 0
	for _, o := range outcomes {
		switch o.kind {
		case outcomeSkip:
			skipped++
		case outcomePass:
			passed[o.tier]++
			executed[o.tier]++
		case outcomeFail:
			executed[o.tier]++
		}
	}
	for _, tier := range []string{tierCore, tierExtended} {
		fmt.Fprintf(stdout, "%s: %d/%d passed\n", tier, passed[tier], executed[tier])
	}
	totalPassed := passed[tierCore] + passed[tierExtended]
	totalExecuted := executed[tierCore] + executed[tierExtended]
	fmt.Fprintf(stdout, "%d/%d passed\n", totalPassed, totalExecuted)
	if skipped > 0 {
		fmt.Fprintf(stdout, "%d skipped (coverage reduced)\n", skipped)
	}

	switch {
	case graderErr != nil:
		fmt.Fprintf(stderr, "grader error: %v\n", graderErr)
		return 2
	case totalExecuted == 0:
		fmt.Fprintln(stderr, "no cases were executed")
		return 2
	case totalPassed < totalExecuted:
		return 1
	}
	return 0
}

type reportWriter struct {
	w   io.Writer
	err error
}

func (rw *reportWriter) Write(p []byte) (int, error) {
	if rw.err != nil {
		return 0, rw.err
	}
	n, err := rw.w.Write(p)
	rw.err = err
	return n, err
}

func pathDiagnosticFailure(stderr, path string) string {
	if strings.Contains(stderr, path) {
		return ""
	}
	return fmt.Sprintf("stderr %q does not mention %s", stderr, path)
}

type pathCheck func(r *result, s *snapshot) string

type caseBuilder struct {
	fx     *fixture
	dir    string
	err    error
	skip   string
	paths  []string
	checks map[string][]pathCheck
}

func newCaseBuilder(fx *fixture, dir string) *caseBuilder {
	return &caseBuilder{fx: fx, dir: dir, checks: map[string][]pathCheck{}}
}

func (b *caseBuilder) path(name string) string { return filepath.Join(b.dir, name) }

func (b *caseBuilder) fail(err error) {
	if b.err == nil {
		b.err = err
	}
}

func (b *caseBuilder) setTimes(name string, times filetime.Times) snapshot {
	if err := os.Chtimes(b.path(name), times.Access, times.Modify); err != nil {
		b.fail(err)
		return snapshot{}
	}
	s, err := inspectMetadata(b.path(name))
	if err != nil {
		b.fail(err)
	}
	return s
}

func (b *caseBuilder) file(name string) snapshot {
	if err := os.WriteFile(b.path(name), []byte(fileContent), 0o600); err != nil {
		b.fail(err)
		return snapshot{}
	}
	s := b.setTimes(name, filetime.Times{Access: seedAccess, Modify: seedModify})
	s.content = fileContent
	return s
}

func (b *caseBuilder) directory(name string, times filetime.Times) snapshot {
	if err := os.Mkdir(b.path(name), 0o700); err != nil {
		b.fail(err)
		return snapshot{}
	}
	return b.setTimes(name, times)
}

func (b *caseBuilder) reference() filetime.Times {
	if err := os.WriteFile(b.path(refName), nil, 0o600); err != nil {
		b.fail(err)
		return filetime.Times{}
	}
	return b.setTimes(refName, filetime.Times{Access: refAccess, Modify: refModify}).times
}

func (b *caseBuilder) link(target, name string) {
	if b.skip != "" {
		return
	}
	if err := os.Symlink(target, b.path(name)); err != nil {
		b.skip = fmt.Sprintf("cannot create symbolic links: %v", err)
	}
}

func (b *caseBuilder) stored(t time.Time) time.Time {
	stored, err := b.fx.store(filetime.Times{Access: t, Modify: t.Add(time.Second)})
	if err != nil {
		b.fail(err)
	}
	return stored.Access
}

func (b *caseBuilder) localStamp(year int, month time.Month, day, hour, minute, second int) time.Time {
	t, skip := localTime(b.fx.loc, year, month, day, hour, minute, second)
	if skip != "" && b.skip == "" {
		b.skip = skip
	}
	if skip != "" {
		return time.Time{}
	}
	return b.stored(t)
}

func (b *caseBuilder) expect(name string, checks ...pathCheck) {
	if _, ok := b.checks[name]; !ok {
		b.paths = append(b.paths, name)
	}
	b.checks[name] = append(b.checks[name], checks...)
}

func (b *caseBuilder) times(access, modify timeWant) pathCheck {
	return func(r *result, s *snapshot) string {
		if !s.exists {
			return "missing"
		}
		return b.fx.timesFailure(r, s, access, modify)
	}
}

func (b *caseBuilder) unchanged(before *snapshot) pathCheck {
	return func(r *result, s *snapshot) string {
		switch {
		case !s.exists:
			return "missing"
		case s.content != before.content || s.size != before.size:
			return fmt.Sprintf("content %q, want it unchanged as %q", s.content, before.content)
		}
		return b.fx.timesFailure(r, s, exactly(before.times.Access), exactly(before.times.Modify))
	}
}

func absent(_ *result, s *snapshot) string {
	if s.exists {
		return "exists, want it absent"
	}
	return ""
}

func emptyFile(_ *result, s *snapshot) string {
	switch {
	case !s.exists:
		return "missing, want an empty regular file"
	case !s.regular || s.size != 0:
		return fmt.Sprintf("regular=%t size=%d, want an empty regular file", s.regular, s.size)
	}
	return ""
}

func isDirectory(_ *result, s *snapshot) string {
	if !s.exists || !s.dir {
		return "want an existing directory"
	}
	return ""
}

func preserved(_ *result, s *snapshot) string {
	switch {
	case !s.exists:
		return "missing"
	case s.content != fileContent || s.size != int64(len(fileContent)):
		return fmt.Sprintf("content %q (size %d), want %q", s.content, s.size, fileContent)
	}
	return ""
}

func (b *caseBuilder) invoke(wantError bool, diagnostic string, args ...string) (touchInvocation, string, error) {
	paths := b.paths
	checks := b.checks
	inv := touchInvocation{args: args, wantError: wantError}
	inv.check = func(r *result) (string, error) {
		if diagnostic != "" {
			if failure := pathDiagnosticFailure(r.stderr, diagnostic); failure != "" {
				return failure, nil
			}
		}
		for _, name := range paths {
			s, err := inspect(b.path(name))
			if err != nil {
				return "", err
			}
			for _, check := range checks[name] {
				if failure := check(r, &s); failure != "" {
					return fmt.Sprintf("%s: %s", name, failure), nil
				}
			}
		}
		return "", nil
	}
	return inv, b.skip, b.err
}

func touchCases(fx *fixture) []touchCase {
	nested := filepath.Join
	builder := func(prepare func(b *caseBuilder) (touchInvocation, string, error)) func(string) (touchInvocation, string, error) {
		return func(dir string) (touchInvocation, string, error) { return prepare(newCaseBuilder(fx, dir)) }
	}
	core := func(name, summary string, prepare func(b *caseBuilder) (touchInvocation, string, error)) touchCase {
		return touchCase{name: name, tier: tierCore, summary: summary, prepare: builder(prepare)}
	}
	extended := func(name, summary string, prepare func(b *caseBuilder) (touchInvocation, string, error)) touchCase {
		return touchCase{name: name, tier: tierExtended, summary: summary, prepare: builder(prepare)}
	}

	selection := func(args []string, access, modify bool) func(b *caseBuilder) (touchInvocation, string, error) {
		return func(b *caseBuilder) (touchInvocation, string, error) {
			before := b.file("target")
			wantAccess, wantModify := exactly(before.times.Access), exactly(before.times.Modify)
			if access {
				wantAccess = current
			}
			if modify {
				wantModify = current
			}
			b.expect("target", preserved, b.times(wantAccess, wantModify))
			return b.invoke(false, "", append(args, "target")...)
		}
	}
	createSelect := func(flag string, access bool) func(b *caseBuilder) (touchInvocation, string, error) {
		return func(b *caseBuilder) (touchInvocation, string, error) {
			ref := b.reference()
			if access {
				b.expect("new", emptyFile, b.times(exactly(ref.Access), current))
			} else {
				b.expect("new", emptyFile, b.times(current, exactly(ref.Modify)))
			}
			return b.invoke(false, "", flag, "-r", refName, "new")
		}
	}
	reference := func(flags []string, access, modify bool) func(b *caseBuilder) (touchInvocation, string, error) {
		return func(b *caseBuilder) (touchInvocation, string, error) {
			ref := b.reference()
			before := b.file("target")
			wantAccess, wantModify := exactly(before.times.Access), exactly(before.times.Modify)
			if access {
				wantAccess = exactly(ref.Access)
			}
			if modify {
				wantModify = exactly(ref.Modify)
			}
			b.expect("target", b.times(wantAccess, wantModify))
			return b.invoke(false, "", append(flags, "-r", refName, "target")...)
		}
	}
	stamp := func(flags []string, value string, year int, month time.Month, day, hour, minute, second int, access, modify bool) func(b *caseBuilder) (touchInvocation, string, error) {
		return func(b *caseBuilder) (touchInvocation, string, error) {
			want := exactly(b.localStamp(year, month, day, hour, minute, second))
			before := b.file("target")
			wantAccess, wantModify := exactly(before.times.Access), exactly(before.times.Modify)
			if access {
				wantAccess = want
			}
			if modify {
				wantModify = want
			}
			b.expect("target", b.times(wantAccess, wantModify))
			return b.invoke(false, "", append(flags, "-t", value, "target")...)
		}
	}
	invalid := func(args ...string) func(b *caseBuilder) (touchInvocation, string, error) {
		return func(b *caseBuilder) (touchInvocation, string, error) {
			b.reference()
			before := b.file("target")
			b.expect("target", b.unchanged(&before))
			b.expect("new", absent)
			return b.invoke(true, "", args...)
		}
	}

	return []touchCase{
		core("create", "creates a missing file empty, with both times set to now", func(b *caseBuilder) (touchInvocation, string, error) {
			b.expect(createCaseTarget, emptyFile, b.times(current, current))
			return b.invoke(false, "", createCaseTarget)
		}),
		core("existing", "keeps an existing file's content and sets both times to now", func(b *caseBuilder) (touchInvocation, string, error) {
			b.file("existing")
			b.expect("existing", preserved, b.times(current, current))
			return b.invoke(false, "", "existing")
		}),
		core("multiple-and-directory", "updates every operand, including a directory", func(b *caseBuilder) (touchInvocation, string, error) {
			b.directory("dir", filetime.Times{Access: seedAccess, Modify: seedModify})
			b.file("two")
			b.expect("one", emptyFile, b.times(current, current))
			b.expect("dir", isDirectory, b.times(current, current))
			b.expect("two", preserved, b.times(current, current))
			return b.invoke(false, "", "one", "dir", "two")
		}),
		core("select-a", "-a changes only the access time", selection([]string{"-a"}, true, false)),
		core("select-m", "-m changes only the modification time", selection([]string{"-m"}, false, true)),
		core("select-am", "-a -m changes both times", selection([]string{"-a", "-m"}, true, true)),
		core("select-ma", "-m -a changes both times", selection([]string{"-m", "-a"}, true, true)),
		core("create-select-a", "-a -r creates the file with the reference access time", createSelect("-a", true)),
		core("create-select-m", "-m -r creates the file with the reference modification time", createSelect("-m", false)),
		core("no-create-missing", "-c skips a missing file without an error", func(b *caseBuilder) (touchInvocation, string, error) {
			b.expect("missing", absent)
			return b.invoke(false, "", "-c", "missing")
		}),
		core("no-create-existing", "-c still updates an existing file", func(b *caseBuilder) (touchInvocation, string, error) {
			b.file("existing")
			b.expect("existing", preserved, b.times(current, current))
			return b.invoke(false, "", "-c", "existing")
		}),
		core("reference-both", "-r copies both reference times to the matching fields", reference(nil, true, true)),
		core("reference-a", "-a -r copies only the reference access time", reference([]string{"-a"}, true, false)),
		core("reference-m", "-m -r copies only the reference modification time", reference([]string{"-m"}, false, true)),
		core("time-8", "-t MMDDhhmm uses the current year", func(b *caseBuilder) (touchInvocation, string, error) {
			stamps := map[int]time.Time{}
			year := time.Now().In(b.fx.loc).Year()
			for _, y := range []int{year - 1, year, year + 1} {
				stamps[y] = b.localStamp(y, time.June, 15, 12, 30, 0)
			}
			b.file("target")
			b.expect("target", func(r *result, s *snapshot) string {
				var want []time.Time
				for _, y := range omittedYears(r.started, r.finished, b.fx.loc) {
					want = append(want, stamps[y])
				}
				return b.fx.timesFailure(r, s, exactly(want...), exactly(want...))
			})
			return b.invoke(false, "", "-t", "06151230", "target")
		}),
		core("time-10", "-t YYMMDDhhmm uses the two-digit year", stamp(nil, "2603021245", 2026, time.March, 2, 12, 45, 0, true, true)),
		core("time-12", "-t CCYYMMDDhhmm uses the four-digit year", stamp(nil, "201907040830", 2019, time.July, 4, 8, 30, 0, true, true)),
		core("time-seconds", "-t accepts optional .ss seconds", stamp(nil, "202606151230.45", 2026, time.June, 15, 12, 30, 45, true, true)),
		core("invalid-option", "an unknown option fails without changing files", invalid("-z", "target", "new")),
		core("invalid-no-files", "no file operand is an error", func(b *caseBuilder) (touchInvocation, string, error) {
			return b.invoke(true, "")
		}),

		extended("no-create-parent", "-c skips a file whose parent directory is missing", func(b *caseBuilder) (touchInvocation, string, error) {
			b.expect("missing", absent)
			return b.invoke(false, "", "-c", nested("missing", "child"))
		}),
		extended("reference-directory", "-r accepts a directory as the reference", func(b *caseBuilder) (touchInvocation, string, error) {
			ref := b.directory("refdir", filetime.Times{Access: refAccess, Modify: refModify}).times
			b.file("target")
			b.expect("target", b.times(exactly(ref.Access), exactly(ref.Modify)))
			return b.invoke(false, "", "-r", "refdir", "target")
		}),
		extended("reference-self", "-r reads the reference once, even when it is also a target", func(b *caseBuilder) (touchInvocation, string, error) {
			ref := b.reference()
			b.file("target")
			b.expect("target", b.times(exactly(ref.Access), exactly(ref.Modify)))
			b.expect(refName, b.times(exactly(ref.Access), exactly(ref.Modify)))
			return b.invoke(false, "", "-r", refName, "target", refName)
		}),
		extended("reference-missing", "a missing reference fails without changing files", func(b *caseBuilder) (touchInvocation, string, error) {
			before := b.file("target")
			b.expect("target", b.unchanged(&before))
			b.expect("new", absent)
			return b.invoke(true, "missing", "-r", "missing", "target", "new")
		}),
		extended("reference-missing-no-create", "a missing reference fails even with -c", func(b *caseBuilder) (touchInvocation, string, error) {
			before := b.file("target")
			b.expect("target", b.unchanged(&before))
			return b.invoke(true, "missing", "-c", "-r", "missing", "target")
		}),
		extended("time-year00", "-t two-digit year 00 means 2000", stamp(nil, "0001151000", 2000, time.January, 15, 10, 0, 0, true, true)),
		extended("time-year26", "-t two-digit year 26 means 2026", stamp(nil, "2610101010", 2026, time.October, 10, 10, 10, 0, true, true)),
		extended("time-leap", "-t accepts February 29 in a leap year", stamp(nil, "202402291200", 2024, time.February, 29, 12, 0, 0, true, true)),
		extended("time-a", "-a -t sets only the access time", stamp([]string{"-a"}, "202606151230", 2026, time.June, 15, 12, 30, 0, true, false)),
		extended("time-m", "-m -t sets only the modification time", stamp([]string{"-m"}, "202606151230", 2026, time.June, 15, 12, 30, 0, false, true)),
		extended("invalid-missing-r", "-r without a value is an error", func(b *caseBuilder) (touchInvocation, string, error) {
			return b.invoke(true, "", "-r")
		}),
		extended("invalid-missing-t", "-t without a value is an error", func(b *caseBuilder) (touchInvocation, string, error) {
			return b.invoke(true, "", "-t")
		}),
		extended("invalid-empty-r", "an empty -r value fails without changing files", invalid("-r", "", "target", "new")),
		extended("invalid-empty-t", "an empty -t value fails without changing files", invalid("-t", "", "target", "new")),
		extended("invalid-conflict", "-r with -t fails without changing files", invalid("-r", refName, "-t", "202606151230", "target", "new")),
		extended("invalid-date", "a malformed -t value fails without changing files", invalid("-t", "2026-06-15", "target", "new")),
		extended("invalid-calendar", "a nonexistent date such as February 30 fails", invalid("-t", "202602301200", "target", "new")),
		extended("name-separator", "-- makes a following -a a file name", func(b *caseBuilder) (touchInvocation, string, error) {
			b.expect("-a", emptyFile, b.times(current, current))
			return b.invoke(false, "", "--", "-a")
		}),
		extended("name-unicode", "a file name with spaces and non-ASCII characters works", func(b *caseBuilder) (touchInvocation, string, error) {
			name := "보고서 초안.txt"
			b.expect(name, emptyFile, b.times(current, current))
			return b.invoke(false, "", name)
		}),
		extended("repeat-r", "the last -r wins", func(b *caseBuilder) (touchInvocation, string, error) {
			ref := b.reference()
			b.file("target")
			b.expect("target", b.times(exactly(ref.Access), exactly(ref.Modify)))
			return b.invoke(false, "", "-r", "missing", "-r", refName, "target")
		}),
		extended("repeat-t", "the last -t wins", func(b *caseBuilder) (touchInvocation, string, error) {
			want := exactly(b.localStamp(2026, time.June, 15, 12, 30, 0))
			b.file("target")
			b.expect("target", b.times(want, want))
			return b.invoke(false, "", "-t", "200001010000", "-t", "202606151230", "target")
		}),
		extended("error-missing-parent", "a missing parent directory is an error without -c", func(b *caseBuilder) (touchInvocation, string, error) {
			b.expect("missing", absent)
			path := nested("missing", "child")
			return b.invoke(true, path, path)
		}),
		extended("error-file-parent", "a regular file used as a directory is an error", func(b *caseBuilder) (touchInvocation, string, error) {
			before := b.file("regular")
			b.expect("regular", b.unchanged(&before))
			path := nested("regular", "child")
			return b.invoke(true, path, path)
		}),
		extended("error-file-parent-no-create", "a regular file used as a directory is an error even with -c", func(b *caseBuilder) (touchInvocation, string, error) {
			before := b.file("regular")
			b.expect("regular", b.unchanged(&before))
			path := nested("regular", "child")
			return b.invoke(true, path, "-c", path)
		}),
		extended("error-continue", "a failing operand does not stop later ones", func(b *caseBuilder) (touchInvocation, string, error) {
			b.expect("after", emptyFile, b.times(current, current))
			path := nested("missing", "child")
			return b.invoke(true, path, path, "after")
		}),
		extended("link-target", "a symbolic link updates its target", func(b *caseBuilder) (touchInvocation, string, error) {
			b.file("target")
			b.link("target", "link")
			b.expect("target", preserved, b.times(current, current))
			return b.invoke(false, "", "link")
		}),
		extended("link-reference", "-r follows a symbolic link to the reference", func(b *caseBuilder) (touchInvocation, string, error) {
			ref := b.reference()
			b.link(refName, "ref-link")
			b.file("target")
			b.expect("target", b.times(exactly(ref.Access), exactly(ref.Modify)))
			return b.invoke(false, "", "-r", "ref-link", "target")
		}),
		extended("link-dangling", "a dangling link creates its target", func(b *caseBuilder) (touchInvocation, string, error) {
			b.link("created", "link")
			b.expect("created", emptyFile, b.times(current, current))
			return b.invoke(false, "", "link")
		}),
		extended("link-dangling-no-create", "-c does not create a dangling link's target", func(b *caseBuilder) (touchInvocation, string, error) {
			b.link("created", "link")
			b.expect("created", absent)
			return b.invoke(false, "", "-c", "link")
		}),
	}
}
