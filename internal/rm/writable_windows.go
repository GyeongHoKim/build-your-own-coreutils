//go:build windows

package rm

import (
	"io/fs"
	"syscall"
)

// writeProtected reports whether fi has the read-only attribute. Windows
// ignores the attribute on directories, so they are never write-protected.
// Access control lists are not consulted; a denial surfaces when removing.
func writeProtected(fi fs.FileInfo) bool {
	if fi.IsDir() {
		return false
	}
	d, ok := fi.Sys().(*syscall.Win32FileAttributeData)
	return ok && d.FileAttributes&syscall.FILE_ATTRIBUTE_READONLY != 0
}
