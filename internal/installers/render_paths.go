package installers

import (
	"path/filepath"
	"strings"
)

func relativePath(root, path string) string {
	rel, _ := filepath.Rel(root, path)
	return "./" + strings.TrimPrefix(filepath.ToSlash(rel), "./")
}
