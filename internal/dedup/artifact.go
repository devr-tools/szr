package dedup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

// MaxArtifactBytes caps a stored raw artifact. Runs whose raw output exceeds
// the cap still dedup on the full hash, but only the first MaxArtifactBytes
// are recoverable via expand and the entry is marked Truncated.
const MaxArtifactBytes = 2 << 20

// HashBytes returns the lowercase hex SHA-256 of data.
func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// WriteArtifact persists the (already cap-limited) artifact bytes into the
// dedup directory and returns the file path plus the stored-bytes hash.
func (s *Store) WriteArtifact(data []byte) (string, string, error) {
	if s == nil || s.dir == "" {
		return "", "", fmt.Errorf("dedup store is not configured")
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return "", "", err
	}
	path, err := writeArtifactFile(s.dir, data)
	if err != nil {
		return "", "", err
	}
	return path, HashBytes(data), nil
}

func writeArtifactFile(dir string, data []byte) (string, error) {
	file, err := os.CreateTemp(dir, "artifact-*.raw")
	if err != nil {
		return "", err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	_ = os.Chmod(file.Name(), 0o644)
	return file.Name(), nil
}

// ReadArtifact returns the stored raw bytes for an entry.
func (s *Store) ReadArtifact(entry Entry) ([]byte, error) {
	if entry.ArtifactPath == "" {
		return nil, fmt.Errorf("dedup entry has no artifact")
	}
	return os.ReadFile(entry.ArtifactPath)
}

// VerifyArtifact reports whether the entry's artifact still exists on disk
// with exactly the bytes that were stored. A reference must never be emitted
// against a missing or corrupt artifact.
func (s *Store) VerifyArtifact(entry Entry) bool {
	if entry.ArtifactHash == "" {
		return false
	}
	data, err := s.ReadArtifact(entry)
	if err != nil {
		return false
	}
	return HashBytes(data) == entry.ArtifactHash
}
