// Package touch creates files and updates their access and modification
// times, the behavior behind the touch command.
package touch

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/GyeongHoKim/build-your-own-coreutils/internal/filetime"
)

// Options selects how Update treats a target.
//
// Access and Modify select the timestamps to change. Setting only one changes
// that timestamp alone; setting both is the same as setting neither, which
// changes both. NoCreate skips missing targets instead of creating them. The
// zero value changes both timestamps and creates missing files.
type Options struct {
	Access   bool
	Modify   bool
	NoCreate bool
}

func (o Options) both() bool { return o.Access == o.Modify }

// ParseTime parses value in the touch -t format [[CC]YY]MMDDhhmm[.ss].
//
// The time is interpreted in now's location, and an omitted year is taken
// from now. Two-digit years 69 through 99 mean 1969 through 1999; 00 through
// 68 mean 2000 through 2068. Every field must be in range: dates are not
// normalized, nonexistent local times are rejected, and seconds stop at 59.
// The result is also checked to be a time that Validate accepts.
func ParseTime(value string, now time.Time) (time.Time, error) {
	t, err := parseTime(value, now)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date format %q: %w", value, err)
	}
	if err := checkTime(t); err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q: %w", value, err)
	}
	return t, nil
}

func parseTime(value string, now time.Time) (time.Time, error) {
	main, secs, hasSecs := strings.Cut(value, ".")
	for _, s := range []string{main, secs} {
		for i := range len(s) {
			if s[i] < '0' || s[i] > '9' {
				return time.Time{}, errors.New("want only ASCII digits")
			}
		}
	}
	if hasSecs && len(secs) != 2 {
		return time.Time{}, errors.New("want exactly two digits of seconds")
	}

	twoDigits := func(s string) (int, string) {
		return int(s[0]-'0')*10 + int(s[1]-'0'), s[2:]
	}

	var year int
	switch len(main) {
	case 8:
		year = now.Year()
	case 10:
		year, main = twoDigits(main)
		if year < 69 {
			year += 2000
		} else {
			year += 1900
		}
	case 12:
		var century, yy int
		century, main = twoDigits(main)
		yy, main = twoDigits(main)
		year = century*100 + yy
	default:
		return time.Time{}, errors.New("want 8, 10, or 12 digits before the seconds")
	}

	month, main := twoDigits(main)
	day, main := twoDigits(main)
	hour, main := twoDigits(main)
	minute, _ := twoDigits(main)
	second := 0
	if hasSecs {
		second, _ = twoDigits(secs)
	}
	if month < 1 || month > 12 || day < 1 || day > 31 || hour > 23 || minute > 59 || second > 59 {
		return time.Time{}, errors.New("field out of range")
	}

	t := time.Date(year, time.Month(month), day, hour, minute, second, 0, now.Location())
	if t.Year() != year || int(t.Month()) != month || t.Day() != day ||
		t.Hour() != hour || t.Minute() != minute || t.Second() != second {
		return time.Time{}, errors.New("no such local time")
	}
	return t, nil
}

// Validate reports an error if a timestamp selected by opts cannot be
// applied exactly. os.Chtimes treats the zero time as "leave unchanged" and
// cannot represent times outside roughly 1678 through 2262, so both are
// rejected. Unselected timestamps are ignored, so a reference file with one
// unusable timestamp can still supply the other.
func Validate(times filetime.Times, opts Options) error {
	if opts.both() || opts.Access {
		if err := checkTime(times.Access); err != nil {
			return fmt.Errorf("access time: %w", err)
		}
	}
	if opts.both() || opts.Modify {
		if err := checkTime(times.Modify); err != nil {
			return fmt.Errorf("modification time: %w", err)
		}
	}
	return nil
}

func checkTime(t time.Time) error {
	if t.IsZero() || !time.Unix(0, t.UnixNano()).Equal(t) {
		return fmt.Errorf("%s is out of range", t.Format(time.RFC3339Nano))
	}
	return nil
}
