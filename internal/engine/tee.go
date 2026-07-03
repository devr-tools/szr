package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func (e *Engine) writeTee(raw string, command []string) (string, error) {
	name := teeFileName(command)
	path := filepath.Join(e.paths.TeeDir, name)
	return path, os.WriteFile(path, []byte(raw), 0o644)
}

type teeCapture struct {
	mu        sync.Mutex
	tempPath  string
	finalPath string
	file      *os.File
	failed    bool
	// retainOnSuccess keeps the finalized capture for successful exits too,
	// so session dedup can hash and archive the full raw stream even when the
	// in-memory capture stopped at the preview limit.
	retainOnSuccess bool
}

func newTeeCapture(dir string, command []string, enabled bool, retainOnSuccess bool) (*teeCapture, error) {
	if !enabled || dir == "" {
		return nil, nil
	}

	file, err := os.CreateTemp(dir, "szr-tee-*.partial")
	if err != nil {
		return nil, err
	}

	return &teeCapture{
		tempPath:        file.Name(),
		finalPath:       filepath.Join(dir, teeFileName(command)),
		file:            file,
		retainOnSuccess: retainOnSuccess,
	}, nil
}

func (t *teeCapture) Write(chunk []byte) {
	if t == nil || len(chunk) == 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.failed || t.file == nil {
		return
	}
	if _, err := t.file.Write(chunk); err != nil {
		t.failed = true
		_ = t.file.Close()
		t.file = nil
		_ = os.Remove(t.tempPath)
	}
}

func (t *teeCapture) Finalize(exitCode int) (string, error) {
	if t == nil {
		return "", nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	return t.finalizeLocked(exitCode)
}

func (t *teeCapture) Discard() {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.retainOnSuccess = false
	_, _ = t.finalizeLocked(0)
}

func (t *teeCapture) finalizeLocked(exitCode int) (string, error) {
	if t.file != nil {
		if err := t.file.Close(); err != nil && !t.failed {
			t.failed = true
		}
		t.file = nil
	}
	if t.failed {
		_ = os.Remove(t.tempPath)
		return "", nil
	}
	if exitCode == 0 && !t.retainOnSuccess {
		_ = os.Remove(t.tempPath)
		return "", nil
	}
	if err := os.Rename(t.tempPath, t.finalPath); err != nil {
		_ = os.Remove(t.tempPath)
		return "", err
	}
	return t.finalPath, nil
}

func teeFileName(command []string) string {
	return fmt.Sprintf("%d_%s.log", time.Now().UnixNano(), sanitizeFileName(strings.Join(command, "_")))
}
