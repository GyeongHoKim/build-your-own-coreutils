//go:build !windows && !linux && !darwin

package filetime

import (
	"errors"
	"fmt"
	"os"
	"runtime"
)

func timesFromInfo(os.FileInfo) (Times, error) {
	return Times{}, fmt.Errorf("access time on %s: %w", runtime.GOOS, errors.ErrUnsupported)
}
