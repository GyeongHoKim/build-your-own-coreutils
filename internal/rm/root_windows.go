//go:build windows

package rm

import (
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// Flags for GetFinalPathNameByHandle, which golang.org/x/sys/windows does
// not define.
const (
	volumeNameDOS  = 0x0
	volumeNameGUID = 0x1
)

// isRootDir reports whether the directory at path, following links and
// junctions, is the root directory of a volume or network share.
//
// It asks Windows for the final path of an open handle rather than parsing
// path, because a junction or volume mount point can lead anywhere and
// filepath.EvalSymlinks does not resolve junctions. The GUID form,
// \\?\Volume{...}\dir, names the volume itself, so the root of a volume is
// recognized whether it is reached through a drive letter, a mount point,
// or a junction. Network shares and some virtual drives have no volume GUID;
// for them the DOS form, such as \\?\UNC\server\share\dir, is used.
func isRootDir(path string, _ fs.FileInfo) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	return isRootFile(f)
}

// isRootHandle reports whether dir, an open directory, is the root of a
// volume or network share. Unlike isRootDir, it examines the directory dir
// refers to, so the answer cannot be invalidated by a path that changes
// later.
func isRootHandle(dir *os.Root) (bool, error) {
	f, err := dir.Open(".")
	if err != nil {
		return false, err
	}
	defer f.Close()
	return isRootFile(f)
}

// isRootFile reports whether the open directory f is the root of a volume
// or network share.
func isRootFile(f *os.File) (bool, error) {
	h := windows.Handle(f.Fd())
	p, err := finalPath(h, volumeNameGUID)
	if err != nil {
		p, err = finalPath(h, volumeNameDOS)
	}
	if err != nil {
		return false, &fs.PathError{Op: "GetFinalPathNameByHandle", Path: f.Name(), Err: err}
	}
	return isVolumeRootPath(p), nil
}

// isVolumeRootPath reports whether p, a final path as returned by
// GetFinalPathNameByHandle, names the root of its volume or share.
// filepath.VolumeName treats \\?\C:, \\?\Volume{...}, and
// \\?\UNC\server\share as volume names.
func isVolumeRootPath(p string) bool {
	return filepath.Dir(p) == p
}

// finalPath returns the final path of the file open as h, with the volume
// named as flags selects.
func finalPath(h windows.Handle, flags uint32) (string, error) {
	size := uint32(windows.MAX_PATH)
	for {
		buf := make([]uint16, size)
		n, err := windows.GetFinalPathNameByHandle(h, &buf[0], size, flags)
		if err != nil {
			return "", err
		}
		if n < size {
			return windows.UTF16ToString(buf[:n]), nil
		}
		// n is the size needed, including the terminating NUL.
		size = n
	}
}
