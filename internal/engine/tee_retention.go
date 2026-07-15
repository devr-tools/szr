package engine

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/teeindex"
)

// teeLimits carries the retention bounds for tee artifacts. Zero fields mean
// "use the default", never "unlimited".
type teeLimits struct {
	maxFileBytes int64
	maxDirFiles  int
	maxDirBytes  int64
}

// staleTeePartialAge is how long an orphaned in-progress capture may linger
// before pruning removes it; live captures are always far younger.
const staleTeePartialAge = time.Hour

func resolveTeeLimits(limits teeLimits) teeLimits {
	if limits.maxFileBytes <= 0 {
		limits.maxFileBytes = int64(config.DefaultTeeMaxFileMB) << 20
	}
	if limits.maxDirFiles <= 0 {
		limits.maxDirFiles = config.DefaultTeeMaxDirFiles
	}
	if limits.maxDirBytes <= 0 {
		limits.maxDirBytes = int64(config.DefaultTeeMaxDirMB) << 20
	}
	return limits
}

// teeLimits resolves the engine's configured tee retention bounds.
func (e *Engine) teeLimits() teeLimits {
	return resolveTeeLimits(teeLimits{
		maxFileBytes: int64(e.config.TeeMaxFileMB) << 20,
		maxDirFiles:  e.config.TeeMaxDirFiles,
		maxDirBytes:  int64(e.config.TeeMaxDirMB) << 20,
	})
}

type teeDirArtifact struct {
	path    string
	size    int64
	modTime time.Time
}

// pruneTeeDir bounds the tee directory to the file-count and total-size caps,
// removing the oldest artifacts first and dropping their index entries so the
// index never serves dangling references. It runs only when a new artifact
// was persisted, keeping the common no-tee path free of directory scans.
func pruneTeeDir(dir string, limits teeLimits) {
	if dir == "" {
		return
	}
	limits = resolveTeeLimits(limits)
	artifacts := collectTeeArtifacts(dir, time.Now())
	removed := removeExcessTeeArtifacts(artifacts, limits)
	dropTeeIndexEntries(dir, removed)
}

func collectTeeArtifacts(dir string, now time.Time) []teeDirArtifact {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	artifacts := make([]teeDirArtifact, 0, len(entries))
	for _, entry := range entries {
		if artifact, ok := teeArtifactFromEntry(dir, entry, now); ok {
			artifacts = append(artifacts, artifact)
		}
	}
	return artifacts
}

// teeArtifactFromEntry classifies one directory entry: completed .log
// artifacts are returned, stale .partial captures are removed, everything
// else is skipped.
func teeArtifactFromEntry(dir string, entry os.DirEntry, now time.Time) (teeDirArtifact, bool) {
	if entry.IsDir() {
		return teeDirArtifact{}, false
	}
	info, err := entry.Info()
	if err != nil {
		return teeDirArtifact{}, false
	}
	path := filepath.Join(dir, entry.Name())
	if strings.HasSuffix(entry.Name(), ".partial") {
		if now.Sub(info.ModTime()) > staleTeePartialAge {
			_ = os.Remove(path)
		}
		return teeDirArtifact{}, false
	}
	if !strings.HasSuffix(entry.Name(), ".log") {
		return teeDirArtifact{}, false
	}
	return teeDirArtifact{path: path, size: info.Size(), modTime: info.ModTime()}, true
}

// removeExcessTeeArtifacts deletes oldest-first until both caps hold. The
// newest artifact always survives, so the run that triggered pruning keeps
// its own reference.
func removeExcessTeeArtifacts(artifacts []teeDirArtifact, limits teeLimits) []string {
	sortTeeArtifactsOldestFirst(artifacts)
	total := totalTeeArtifactBytes(artifacts)
	count := len(artifacts)
	removed := make([]string, 0)
	for _, artifact := range oldestTeeArtifacts(artifacts) {
		if count <= limits.maxDirFiles && total <= limits.maxDirBytes {
			break
		}
		if err := os.Remove(artifact.path); err != nil && !os.IsNotExist(err) {
			continue
		}
		removed = append(removed, artifact.path)
		count--
		total -= artifact.size
	}
	return removed
}

func sortTeeArtifactsOldestFirst(artifacts []teeDirArtifact) {
	sort.Slice(artifacts, func(i, j int) bool {
		if artifacts[i].modTime.Equal(artifacts[j].modTime) {
			return artifacts[i].path < artifacts[j].path
		}
		return artifacts[i].modTime.Before(artifacts[j].modTime)
	})
}

func totalTeeArtifactBytes(artifacts []teeDirArtifact) int64 {
	var total int64
	for _, artifact := range artifacts {
		total += artifact.size
	}
	return total
}

func oldestTeeArtifacts(artifacts []teeDirArtifact) []teeDirArtifact {
	if len(artifacts) == 0 {
		return nil
	}
	return artifacts[:len(artifacts)-1]
}

// dropTeeIndexEntries rewrites the tee index without the pruned files so a
// lookup never resolves to a path pruning removed.
func dropTeeIndexEntries(dir string, removedPaths []string) {
	if len(removedPaths) == 0 {
		return
	}
	store := teeindex.New(dir)
	entries, err := store.LoadAll()
	if err != nil || len(entries) == 0 {
		return
	}
	survivors := filterRemovedTeeIndexEntries(entries, removedPaths)
	if len(survivors) != len(entries) {
		_ = store.Replace(survivors)
	}
}

func filterRemovedTeeIndexEntries(entries []teeindex.Entry, removedPaths []string) []teeindex.Entry {
	removed := make(map[string]struct{}, len(removedPaths))
	for _, path := range removedPaths {
		removed[path] = struct{}{}
	}
	survivors := make([]teeindex.Entry, 0, len(entries))
	for _, entry := range entries {
		if _, gone := removed[entry.Path]; !gone {
			survivors = append(survivors, entry)
		}
	}
	return survivors
}
