package touch

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/GyeongHoKim/build-your-own-coreutils/internal/filetime"
)

// Update sets the timestamps of path selected by opts to the corresponding
// values in times, following symbolic links. Unselected timestamps are left
// unchanged.
//
// A missing path is created as an empty regular file, without creating
// parent directories, unless opts.NoCreate is set, in which case it is
// skipped without error. A regular file used as a parent directory is always
// an error. Existing contents are never truncated. If the file is created
// but its timestamps cannot be set, the file is kept and the error returned.
func Update(path string, times filetime.Times, opts Options) error {
	if err := Validate(times, opts); err != nil {
		return err
	}
	access, modify := times.Access, times.Modify
	if !opts.both() {
		if !opts.Access {
			access = time.Time{}
		}
		if !opts.Modify {
			modify = time.Time{}
		}
	}

	err := os.Chtimes(path, access, modify)
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if opts.NoCreate {
		return checkParents(path)
	}
	if err := create(path); err != nil {
		return err
	}
	return os.Chtimes(path, access, modify)
}

func create(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0o666)
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}
	return nil
}

func checkParents(path string) error {
	resolved := resolvedPath(path)
	volume := len(filepath.VolumeName(resolved))
	for i := volume + 1; i < len(resolved); i++ {
		if !os.IsPathSeparator(resolved[i]) || os.IsPathSeparator(resolved[i-1]) {
			continue
		}
		parent := resolved[:i]
		info, err := os.Stat(parent)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return nil
		case err != nil:
			return err
		case !info.IsDir():
			return &fs.PathError{Op: "stat", Path: path, Err: fmt.Errorf("%s is not a directory", parent)}
		}
	}
	return nil
}
