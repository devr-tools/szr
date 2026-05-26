package localcmd

import (
	"os"
	"path/filepath"
	"strings"
)

func pathDepth(value string) int {
	value = filepath.Clean(value)
	if value == "." || value == string(filepath.Separator) {
		return 0
	}
	return len(strings.Split(value, string(filepath.Separator)))
}

func filepathWalk(root string, fn filepath.WalkFunc) error {
	return filepath.Walk(root, fn)
}

func skipDir() error {
	return filepath.SkipDir
}

var _ os.FileInfo
