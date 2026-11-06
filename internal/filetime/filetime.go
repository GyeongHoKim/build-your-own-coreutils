// Package filetime reads file access and modification times without losing
// precision or range.
//
// The standard library exposes only the modification time portably. This
// package decodes the access time from operating-system metadata as well, and
// returns the stored instants even when they lie outside the range that
// os.Chtimes can set. Callers decide which timestamps they use and whether
// those values are acceptable.
package filetime

import (
	"fmt"
	"os"
	"time"
)

// Times holds a file's access and modification times.
type Times struct {
	Access time.Time
	Modify time.Time
}

// Read returns the access and modification times of path, following
// symbolic links. Lookup errors wrap the underlying error, so callers can
// test for conditions such as fs.ErrNotExist.
func Read(path string) (Times, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Times{}, fmt.Errorf("read timestamps: %w", err)
	}
	times, err := timesFromInfo(info)
	if err != nil {
		return Times{}, fmt.Errorf("read timestamps of %s: %w", path, err)
	}
	return times, nil
}
