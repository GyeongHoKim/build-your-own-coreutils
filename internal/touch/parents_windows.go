package touch

import (
	"path/filepath"
	"strings"
)

func resolvedPath(path string) string {
	if strings.HasPrefix(path, `\?\`) {
		return path
	}
	return filepath.Clean(path)
}
