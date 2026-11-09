//go:build unix

package rm

import (
	"io/fs"
	"os"
)

// isRootDir reports whether the directory at path, described by fi, is the
// root directory "/". It compares device and inode numbers, as GNU rm's
// --preserve-root does, so the root is recognized however it is reached:
// through a symbolic link or a bind mount.
func isRootDir(_ string, fi fs.FileInfo) (bool, error) {
	root, err := os.Stat("/")
	if err != nil {
		return false, err
	}
	return os.SameFile(fi, root), nil
}
