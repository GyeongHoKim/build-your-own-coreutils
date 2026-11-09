package rm

import (
	"errors"
	"io/fs"
	"os"
)

// walker carries the state shared by every entry that Remove processes.
type walker struct {
	opts Options
	cb   Callbacks
}

// remove applies POSIX rm steps 1, 3, and 4 to the entry name inside root.
// path is the user-visible name of the entry. It reports whether the entry
// was removed or the removal was declined.
func (w *walker) remove(root *os.Root, name, path string) bool {
	fi, err := root.Lstat(name)
	if err != nil {
		return w.statFailed(path, err)
	}
	if fi.IsDir() && !w.opts.Recursive && !w.opts.Dirs {
		w.failed(path, ErrIsDir)
		return false
	}

	wp := writeProtected(fi)
	if !w.opts.Force && ((wp && w.opts.Terminal) || w.opts.Interactive) {
		if !w.confirm(removeQuestion(fi, path, wp)) {
			return true
		}
	}

	if err := root.Remove(name); err != nil {
		return w.statFailed(path, err)
	}
	w.removed(path, fi.IsDir())
	return true
}

// statFailed reports err for path, except that with Force a missing entry is
// silently accepted, as POSIX requires.
func (w *walker) statFailed(path string, err error) bool {
	if w.opts.Force && errors.Is(err, fs.ErrNotExist) {
		return true
	}
	w.failed(path, err)
	return false
}

func (w *walker) confirm(question string) bool {
	return w.cb.Confirm != nil && w.cb.Confirm(question)
}

func (w *walker) removed(path string, dir bool) {
	if w.cb.Removed != nil {
		w.cb.Removed(path, dir)
	}
}

// failed reports err for path, dropping the *fs.PathError wrapper whose
// path is relative to an internal root rather than what the user typed.
func (w *walker) failed(path string, err error) {
	if pe, ok := errors.AsType[*fs.PathError](err); ok {
		err = pe.Err
	}
	if w.cb.Failed != nil {
		w.cb.Failed(path, err)
	}
}
