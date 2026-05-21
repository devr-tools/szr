package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (e *Engine) writeTee(raw string, command []string) (string, error) {
	name := fmt.Sprintf("%d_%s.log", time.Now().Unix(), sanitizeFileName(strings.Join(command, "_")))
	path := filepath.Join(e.paths.TeeDir, name)
	return path, os.WriteFile(path, []byte(raw), 0o644)
}
