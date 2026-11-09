//go:build unix

package rm

import (
	"io/fs"
	"os"
	"slices"
	"syscall"
)

// writeProtected reports whether the effective user lacks write permission
// on fi according to its permission bits. The superuser is never denied, and
// symbolic links have no meaningful permissions.
func writeProtected(fi fs.FileInfo) bool {
	if fi.Mode()&fs.ModeSymlink != 0 {
		return false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	uid := os.Geteuid()
	if uid == 0 {
		return false
	}
	perm := fi.Mode().Perm()
	switch {
	case int(st.Uid) == uid:
		return perm&0o200 == 0
	case inGroup(int(st.Gid)):
		return perm&0o020 == 0
	default:
		return perm&0o002 == 0
	}
}

// inGroup reports whether gid is the effective or a supplementary group of
// the process.
func inGroup(gid int) bool {
	if gid == os.Getegid() {
		return true
	}
	groups, err := os.Getgroups()
	return err == nil && slices.Contains(groups, gid)
}
