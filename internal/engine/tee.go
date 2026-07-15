package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func (e *Engine) writeTee(raw string, command []string) (string, error) {
	limits := e.teeLimits()
	name := teeFileName(command)
	path := filepath.Join(e.paths.TeeDir, name)
	if err := os.WriteFile(path, CapTeeContent([]byte(raw), limits.maxFileBytes), 0o644); err != nil {
		return path, err
	}
	pruneTeeDir(e.paths.TeeDir, limits)
	return path, nil
}

// CapTeeContent bounds a tee artifact to maxBytes by keeping the head and
// tail of the stream around a truncation marker. The marker embeds the full
// stream's SHA-256, so a capped file remains exactly as discriminating as the
// stream it stands in for.
func CapTeeContent(raw []byte, maxBytes int64) []byte {
	if maxBytes <= 0 || int64(len(raw)) <= maxBytes {
		return raw
	}
	headLimit, tailLimit := teeHeadTailLimits(maxBytes)
	dropped := int64(len(raw)) - headLimit - tailLimit
	sum := sha256.Sum256(raw)
	marker := teeTruncationMarker(dropped, int64(len(raw)), hex.EncodeToString(sum[:]))
	capped := make([]byte, 0, maxBytes+int64(len(marker)))
	capped = append(capped, raw[:headLimit]...)
	capped = append(capped, marker...)
	return append(capped, raw[int64(len(raw))-tailLimit:]...)
}

func teeHeadTailLimits(maxBytes int64) (int64, int64) {
	head := maxBytes / 2
	return head, maxBytes - head
}

func teeTruncationMarker(dropped, total int64, fullHash string) string {
	return fmt.Sprintf("\n[szr tee truncated: %d bytes omitted of %d total, full stream sha256 %s]\n", dropped, total, fullHash)
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
	limits          teeLimits
	headLimit       int64
	tailLimit       int64
	// written counts the head bytes already persisted to the file.
	written int64
	// total counts every stream byte, capped or not.
	total int64
	// tail holds a rolling window of the newest bytes past the head budget.
	tail []byte
	// hasher digests the FULL stream so capping the file's contents never
	// weakens session dedup's byte-identical detection.
	hasher hash.Hash
}

func newTeeCapture(dir string, command []string, enabled bool, retainOnSuccess bool, limits teeLimits) (*teeCapture, error) {
	if !enabled || dir == "" {
		return nil, nil
	}

	file, err := os.CreateTemp(dir, "szr-tee-*.partial")
	if err != nil {
		return nil, err
	}

	limits = resolveTeeLimits(limits)
	headLimit, tailLimit := teeHeadTailLimits(limits.maxFileBytes)
	return &teeCapture{
		tempPath:        file.Name(),
		finalPath:       filepath.Join(dir, teeFileName(command)),
		file:            file,
		retainOnSuccess: retainOnSuccess,
		limits:          limits,
		headLimit:       headLimit,
		tailLimit:       tailLimit,
		hasher:          sha256.New(),
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
	t.total += int64(len(chunk))
	_, _ = t.hasher.Write(chunk)
	head := chunk
	if remaining := t.headLimit - t.written; remaining < int64(len(chunk)) {
		if remaining < 0 {
			remaining = 0
		}
		head = chunk[:remaining]
	}
	if len(head) > 0 {
		if _, err := t.file.Write(head); err != nil {
			t.abortLocked()
			return
		}
		t.written += int64(len(head))
	}
	if rest := chunk[len(head):]; len(rest) > 0 {
		t.tail = appendTeeTail(t.tail, rest, int(t.tailLimit))
	}
}

// appendTeeTail keeps a rolling window of the newest bytes. The buffer may
// grow to twice the limit before compacting so large streams stay O(1)
// amortized per byte.
func appendTeeTail(tail []byte, chunk []byte, limit int) []byte {
	if limit <= 0 {
		return nil
	}
	if len(chunk) >= limit {
		return append(tail[:0], chunk[len(chunk)-limit:]...)
	}
	tail = append(tail, chunk...)
	if len(tail) > 2*limit {
		tail = append(tail[:0], tail[len(tail)-limit:]...)
	}
	return tail
}

func (t *teeCapture) abortLocked() {
	t.failed = true
	_ = t.file.Close()
	t.file = nil
	_ = os.Remove(t.tempPath)
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
	keep := exitCode != 0 || t.retainOnSuccess
	t.closeFileLocked(keep)
	if t.failed || !keep {
		_ = os.Remove(t.tempPath)
		return "", nil
	}
	if err := os.Rename(t.tempPath, t.finalPath); err != nil {
		_ = os.Remove(t.tempPath)
		return "", err
	}
	if exitCode != 0 {
		// Failure artifacts are the tee files that persist; bounding the
		// directory here keeps pruning off the hot success path, where the
		// retained capture is removed again right after session dedup.
		pruneTeeDir(filepath.Dir(t.finalPath), t.limits)
	}
	return t.finalPath, nil
}

func (t *teeCapture) closeFileLocked(flushTail bool) {
	if t.file == nil {
		return
	}
	if flushTail && !t.failed {
		if err := t.flushTailLocked(); err != nil {
			t.failed = true
		}
	}
	if err := t.file.Close(); err != nil && !t.failed {
		t.failed = true
	}
	t.file = nil
}

// flushTailLocked appends the retained tail, behind a truncation marker when
// bytes were dropped. The marker carries the full-stream hash computed
// incrementally during Write, so hashing the capped file stays exactly as
// discriminating as hashing the full stream.
func (t *teeCapture) flushTailLocked() error {
	overflow := t.total - t.written
	if overflow <= 0 {
		return nil
	}
	tail := t.tail
	if int64(len(tail)) > t.tailLimit {
		tail = tail[int64(len(tail))-t.tailLimit:]
	}
	if dropped := overflow - int64(len(tail)); dropped > 0 {
		marker := teeTruncationMarker(dropped, t.total, hex.EncodeToString(t.hasher.Sum(nil)))
		if _, err := t.file.WriteString(marker); err != nil {
			return err
		}
	}
	_, err := t.file.Write(tail)
	return err
}

func teeFileName(command []string) string {
	return fmt.Sprintf("%d_%s.log", time.Now().UnixNano(), sanitizeFileName(strings.Join(command, "_")))
}
