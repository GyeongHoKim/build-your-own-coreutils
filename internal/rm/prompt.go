package rm

import "io/fs"

// removeQuestion returns the question asked before removing the entry at
// path, described by fi.
func removeQuestion(fi fs.FileInfo, path string, writeProtected bool) string {
	return question("remove", kind(fi), path, writeProtected)
}

// descendQuestion returns the question asked before removing the entries of
// the directory at path.
func descendQuestion(path string, writeProtected bool) string {
	return question("descend into", "directory", path, writeProtected)
}

// question returns "<verb> [write-protected ]<kind> '<path>'?".
func question(verb, kind, path string, writeProtected bool) string {
	q := verb + " "
	if writeProtected {
		q += "write-protected "
	}
	return q + kind + " '" + path + "'?"
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
