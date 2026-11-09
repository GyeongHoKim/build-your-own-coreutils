// Package rm removes files and directories following the POSIX rm utility.
package rm

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// Options selects the behavior of Remove.
type Options struct {
	Recursive, Dirs, Force, Interactive, Verbose bool
	Terminal                                     bool // standard input is a terminal; enables write-protected prompts
	Jobs                                         int  // max concurrent removals; < 1 means runtime.NumCPU(); Interactive forces 1
}

// Callbacks receives questions and results from Remove. Callbacks are never
// called concurrently. A nil Confirm answers "no"; a nil Removed or Failed is
// ignored.
type Callbacks struct {
	// Confirm asks question and reports whether the answer is affirmative.
	Confirm func(question string) bool
	// Removed reports that path was removed; dir tells whether it was a directory.
	Removed func(path string, dir bool)
	// Failed reports that path could not be removed. err carries no
	// *fs.PathError wrapper, so its message does not repeat the path.
	Failed func(path string, err error)
}

// Errors passed to Callbacks.Failed for operands that rm refuses to remove.
// The wording follows the strerror and GNU rm messages users already know.
var (
	ErrIsDir  = errors.New("Is a directory")  //nolint:staticcheck // strerror wording
	ErrNotDir = errors.New("Not a directory") //nolint:staticcheck // strerror wording
	ErrDot    = errors.New("refusing to remove '.' or '..'")
	ErrRoot   = errors.New("refusing to remove the root directory")
)

// Remove applies the POSIX rm algorithm to each operand and reports whether
// every requested entry was removed or declined.
func Remove(operands []string, opts Options, cb Callbacks) bool {
	w := &walker{opts: opts, cb: cb}
	ok := true
	for _, op := range operands {
		if !w.operand(op) {
			ok = false
		}
	}
	return ok
}

// operand checks op and removes the entry it names. Refusals happen before
// anything is changed on the file system.
func (w *walker) operand(op string) bool {
	if op == "" {
		// POSIX pathname resolution fails on an empty pathname with ENOENT.
		return w.statFailed(op, syscall.ENOENT)
	}
	trimmed := trimSeparators(op)
	if base := filepath.Base(trimmed); base == "." || base == ".." {
		w.failed(op, ErrDot)
		return false
	}
	trailing := trimmed != op
	isroot, err := isRoot(op, trimmed, trailing)
	if err != nil {
		// Root-ness is unknown, so nothing is removed. A missing operand
		// is reported as usual, and silently accepted with Force.
		return w.statFailed(op, err)
	}
	if isroot {
		w.failed(op, ErrRoot)
		return false
	}

	target := trimmed
	if trailing {
		// POSIX pathname resolution: "name/" names a directory, following
		// name if it is a symbolic link.
		fi, err := os.Lstat(trimmed)
		if err != nil {
			return w.statFailed(op, err)
		}
		if !fi.IsDir() {
			fi, err := os.Stat(trimmed)
			if err != nil {
				return w.statFailed(op, err)
			}
			if !fi.IsDir() {
				w.failed(op, ErrNotDir)
				return false
			}
			if target, err = filepath.EvalSymlinks(trimmed); err != nil {
				return w.statFailed(op, err)
			}
		}
	}

	dir, name := splitOperand(target)
	root, err := os.OpenRoot(dir)
	if err != nil {
		return w.statFailed(op, err)
	}
	defer root.Close()
	return w.remove(root, name, op)
}

// splitOperand splits p after its last separator into the directory that
// contains the entry and the entry's name. Unlike filepath.Dir, it does not
// clean the directory: in "link/../f", the file system must resolve ".."
// after following link, and cleaning would turn the directory into ".".
func splitOperand(p string) (dir, name string) {
	vol := len(filepath.VolumeName(p))
	i := len(p)
	for i > vol && !os.IsPathSeparator(p[i-1]) {
		i--
	}
	dir, name = p[:i], p[i:]
	if dir == "" {
		dir = "."
	}
	return dir, name
}

// trimSeparators removes trailing path separators from p, keeping the one
// that names a root directory.
func trimSeparators(p string) string {
	vol := len(filepath.VolumeName(p))
	i := len(p)
	for i > vol+1 && os.IsPathSeparator(p[i-1]) {
		i--
	}
	return p[:i]
}

// isRoot reports whether op, with trailing separators removed in trimmed,
// names the root directory of a file system. The last component is followed
// only when op ends in a separator: without one, op names the entry itself,
// such as a symbolic link to the root, which is removed as a link.
//
// Spelling alone cannot tell: "link/" names whatever directory link leads
// to, and filepath.EvalSymlinks does not resolve Windows junctions. So a
// directory operand is also compared by identity with the root; see
// isRootDir. An error means root-ness could not be decided, and the caller
// must not remove anything.
func isRoot(op, trimmed string, trailing bool) (bool, error) {
	if a, err := filepath.Abs(op); err == nil && filepath.Dir(a) == a {
		return true, nil
	}
	stat := os.Lstat
	if trailing {
		stat = os.Stat
	}
	fi, err := stat(trimmed)
	if err != nil {
		return false, err
	}
	if !fi.IsDir() {
		// A file, or a link that is removed as an entry.
		return false, nil
	}
	return isRootDir(trimmed, fi)
}
