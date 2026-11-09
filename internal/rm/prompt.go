package rm

import "io/fs"

// removeQuestion returns the question asked before removing the entry at
// path, described by fi.
func removeQuestion(fi fs.FileInfo, path string, writeProtected bool) string {
	q := "remove "
	if writeProtected {
		q += "write-protected "
	}
	return q + kind(fi) + " '" + path + "'?"
}

// kind describes the type of a file in the words of GNU rm prompts.
func kind(fi fs.FileInfo) string {
	switch m := fi.Mode(); {
	case m.IsRegular() && fi.Size() == 0:
		return "regular empty file"
	case m.IsRegular():
		return "regular file"
	case m.IsDir():
		return "directory"
	case m&fs.ModeSymlink != 0:
		return "symbolic link"
	default:
		return "file"
	}
}
