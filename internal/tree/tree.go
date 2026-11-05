// Package tree lists directories as a depth-indented tree, like tree(1).
package tree

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	branch     = "├── "
	lastBranch = "└── "
	pipe       = "│   "
	blank      = "    "

	errorOpening = "  [error opening dir]"
)

// Options selects which entries Print lists and in what order.
type Options struct {
	All       bool
	DirsOnly  bool
	DirsFirst bool
	MaxDepth  int
}

// Counts is the number of directories and files listed. Like tree(1), the
// root directory is counted only when at least one of its entries is listed.
type Counts struct {
	Dirs  int
	Files int
}

// Add returns the sum of c and other.
func (c Counts) Add(other Counts) Counts {
	return Counts{Dirs: c.Dirs + other.Dirs, Files: c.Files + other.Files}
}

// Print writes the tree rooted at root to w and returns what it listed.
// Unreadable directories are annotated and skipped; their errors are joined
// into the returned error. Printing stops at the first write error.
func Print(w io.Writer, root string, opts Options) (Counts, error) {
	entries, err := os.ReadDir(root)
	line := root
	if err != nil {
		line += errorOpening
	}
	if _, werr := fmt.Fprintln(w, line); werr != nil {
		return Counts{}, werr
	}

	p := printer{w: w, opts: opts}
	if err != nil {
		// tree(1) lists a non-directory argument as a file.
		if info, serr := os.Stat(root); serr == nil && !info.IsDir() {
			p.counts.Files++
		}
	}
	p.list(root, entries, "", 1)
	if err == nil && p.counts != (Counts{}) {
		p.counts.Dirs++
	}
	return p.counts, errors.Join(err, p.err)
}

// Report writes the summary line, such as "3 directories, 5 files".
func Report(w io.Writer, c Counts, dirsOnly bool) error {
	var b strings.Builder
	fmt.Fprintf(&b, "\n%d %s", c.Dirs, plural(c.Dirs, "directory", "directories"))
	if !dirsOnly {
		fmt.Fprintf(&b, ", %d %s", c.Files, plural(c.Files, "file", "files"))
	}
	b.WriteByte('\n')
	_, err := io.WriteString(w, b.String())
	return err
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

type printer struct {
	w      io.Writer
	opts   Options
	counts Counts
	err    error
	stop   bool // a write failed; stop listing
}

// entry is a directory entry with its path and whether tree(1) treats it as a
// directory (a symlink to a directory is one, but is never descended into).
type entry struct {
	os.DirEntry
	path  string
	isDir bool
}

func (p *printer) list(dir string, dirEntries []os.DirEntry, prefix string, depth int) {
	entries := p.filter(dir, dirEntries)

	for i, e := range entries {
		isLast := i == len(entries)-1
		connector, childPrefix := branch, prefix+pipe
		if isLast {
			connector, childPrefix = lastBranch, prefix+blank
		}

		name := e.Name()
		if e.Type()&os.ModeSymlink != 0 {
			if target, err := os.Readlink(e.path); err == nil {
				name += " -> " + target
			}
		}

		var children []os.DirEntry
		descend := e.IsDir() && (p.opts.MaxDepth <= 0 || depth < p.opts.MaxDepth)
		if descend {
			var err error
			if children, err = os.ReadDir(e.path); err != nil {
				p.err = errors.Join(p.err, err)
				name += errorOpening
			}
		}

		if _, err := fmt.Fprintln(p.w, prefix+connector+name); err != nil {
			p.err = errors.Join(p.err, err)
			p.stop = true
			return
		}

		if e.isDir {
			p.counts.Dirs++
		} else {
			p.counts.Files++
		}
		if descend {
			p.list(e.path, children, childPrefix, depth+1)
			if p.stop {
				return
			}
		}
	}
}

func (p *printer) filter(dir string, dirEntries []os.DirEntry) []entry {
	out := make([]entry, 0, len(dirEntries))
	for _, de := range dirEntries {
		if !p.opts.All && strings.HasPrefix(de.Name(), ".") {
			continue
		}
		e := entry{DirEntry: de, path: filepath.Join(dir, de.Name())}
		e.isDir = de.IsDir()
		if !e.isDir && de.Type()&os.ModeSymlink != 0 {
			info, err := os.Stat(e.path)
			e.isDir = err == nil && info.IsDir()
		}
		if p.opts.DirsOnly && !e.isDir {
			continue
		}
		out = append(out, e)
	}
	if p.opts.DirsFirst {
		slices.SortStableFunc(out, func(a, b entry) int {
			switch {
			case a.isDir == b.isDir:
				return 0
			case a.isDir:
				return -1
			default:
				return 1
			}
		})
	}
	return out
}
