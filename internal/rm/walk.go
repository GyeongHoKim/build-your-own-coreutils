package rm

import (
	"errors"
	"io/fs"
	"os"
	"slices"
)

// walker carries the state shared by every entry that Remove processes.
type walker struct {
	opts Options
	cb   Callbacks
}

// beforeRemove and beforeOpen, when set by tests, are called with the name
// of an entry just before it is removed or opened as a directory, to
// simulate concurrent changes.
var beforeRemove, beforeOpen func(name string)

// errReplaced reports a directory that was replaced, for example by a
// symbolic link, between being examined and being opened.
var errReplaced = errors.New("directory changed during removal")

// entry applies POSIX rm steps 1 to 4 to the entry name inside parent.
// display is the user-visible path of the entry. It reports whether the entry
// was removed or its removal was declined.
func (w *walker) entry(parent *os.Root, name, display string) bool {
	fi, err := parent.Lstat(name)
	if err != nil {
		return w.statFailed(display, err)
	}
	// Lstat does not follow links, so only a real directory is descended
	// into. Symbolic links and junctions, like other name-surrogate reparse
	// points, are not reported as directories and are removed as entries.
	// Other directory reparse points, such as OneDrive placeholders, are
	// ModeDir|ModeIrregular and are traversed: they redirect nowhere.
	switch {
	case !fi.IsDir():
	case w.opts.Recursive:
		return w.tree(fi, display,
			func() (*os.Root, error) {
				if beforeOpen != nil {
					beforeOpen(name)
				}
				return parent.OpenRoot(name)
			},
			func() error { return unlink(parent, name) })
	case !w.opts.Dirs:
		w.failed(display, ErrIsDir)
		return false
	}
	return w.single(fi, display, func() error { return unlink(parent, name) })
}

// single applies steps 3 and 4 to the entry at display, described by fi:
// it prompts if required, then calls remove.
func (w *walker) single(fi fs.FileInfo, display string, remove func() error) bool {
	wp := writeProtected(fi)
	if w.prompting(wp) && !w.confirm(removeQuestion(fi, display, wp)) {
		return true
	}
	if err := remove(); err != nil {
		return w.statFailed(display, err)
	}
	w.removed(display, fi.IsDir())
	return true
}

// tree applies step 2 to the directory at display, described by fi. open
// returns a handle to the directory, and rmdir removes it in step 4. Removal
// is attempted even when an entry inside was declined or failed, as POSIX
// requires; it then fails because the directory is not empty.
func (w *walker) tree(fi fs.FileInfo, display string, open func() (*os.Root, error), rmdir func() error) bool {
	dir, listErr := open()
	if listErr == nil && !sameDir(dir, fi) {
		// open followed a link that replaced the directory after it was
		// examined; descending would remove entries elsewhere.
		closeRoot(dir)
		w.failed(display, errReplaced)
		return false
	}
	var names []string
	if listErr == nil {
		names, listErr = readNames(dir)
	}
	if errors.Is(listErr, fs.ErrNotExist) {
		closeRoot(dir)
		return w.statFailed(display, listErr)
	}

	// Step 2b. Only a directory known to be empty skips to step 2d.
	if len(names) > 0 || listErr != nil {
		wp := writeProtected(fi)
		if w.prompting(wp) && !w.confirm(descendQuestion(display, wp)) {
			closeRoot(dir)
			return true
		}
	}
	ok := true
	for _, name := range names {
		if !w.entry(dir, name, join(display, name)) {
			ok = false
		}
	}
	closeRoot(dir)

	// Step 2d.
	if w.opts.Interactive && !w.opts.Force &&
		!w.confirm(removeQuestion(fi, display, writeProtected(fi))) {
		return ok
	}
	if err := rmdir(); err != nil {
		// An unlistable directory that is not empty fails to be removed;
		// the listing error then explains why better than ENOTEMPTY.
		if listErr != nil {
			err = listErr
		}
		return w.statFailed(display, err) && ok
	}
	w.removed(display, true)
	return ok
}

// prompting reports whether steps 2b and 3 ask before acting on an entry
// whose write protection is wp.
func (w *walker) prompting(wp bool) bool {
	return !w.opts.Force && ((wp && w.opts.Terminal) || w.opts.Interactive)
}

// sameDir reports whether dir is the directory described by fi. A failure
// to tell counts as different.
func sameDir(dir *os.Root, fi fs.FileInfo) bool {
	st, err := dir.Stat(".")
	return err == nil && os.SameFile(fi, st)
}

// readNames returns the sorted names of the entries in dir. Every name is
// read before any entry is removed, so removal does not disturb the listing.
func readNames(dir *os.Root) ([]string, error) {
	f, err := dir.Open(".")
	if err != nil {
		return nil, err
	}
	names, err := f.Readdirnames(-1)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, err
	}
	slices.Sort(names)
	return names, nil
}

// join appends name to the user-visible directory path dir, keeping the
// spelling of dir, such as "./d" or "d/", as GNU rm does.
func join(dir, name string) string {
	if os.IsPathSeparator(dir[len(dir)-1]) {
		return dir + name
	}
	return dir + string(os.PathSeparator) + name //nolint:gocritic // filepath.Join would clean the spelling the user typed
}

func unlink(parent *os.Root, name string) error {
	if beforeRemove != nil {
		beforeRemove(name)
	}
	return parent.Remove(name)
}

// closeRoot closes r if it is open. Close errors are ignored: nothing was
// written through r, and removal reports its own errors.
func closeRoot(r *os.Root) {
	if r != nil {
		_ = r.Close()
	}
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
