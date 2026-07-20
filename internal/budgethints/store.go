// Package budgethints stores untrusted, aggregate budget recommendations from
// a gateway. It deliberately contains no command text, output, paths, or
// credentials; callers identify a command only by its local fingerprint.
package budgethints

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const CurrentVersion = 1

type Direction string

const (
	DirectionTighten Direction = "tighten"
	DirectionLoosen  Direction = "loosen"
)

// Target is a requested output cap. A zero field leaves the corresponding
// local cap unchanged.
type Target struct {
	MaxLines  int `json:"max_lines,omitempty"`
	MaxBytes  int `json:"max_bytes,omitempty"`
	MaxTokens int `json:"max_tokens,omitempty"`
}

// Hint is intentionally evidence-bearing and short-lived. Profile is
// required; Fingerprint is optional, and narrows a profile-level hint to one
// locally computed command fingerprint.
type Hint struct {
	Version     int       `json:"version"`
	Profile     string    `json:"profile"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	Direction   Direction `json:"direction"`
	Samples     int       `json:"samples"`
	ExpiresAt   time.Time `json:"expires_at"`
	Suggested   Target    `json:"suggested"`
}

// Store is a local JSON document. It is read-only on the command path; a
// future authenticated gateway client may replace it out of band.
type Store struct{ path string }

func New(path string) *Store { return &Store{path: path} }

// Replace validates every supplied hint before atomically replacing the
// store. Files are owner-readable only.
//
//nolint:maintidx // Atomic replace follows the filesystem safety sequence.
func (s *Store) Replace(hints []Hint) error {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return errors.New("budget hint store is not configured")
	}
	for _, hint := range hints {
		if err := Validate(hint); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	body, err := json.Marshal(hints)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(s.path), ".budget-hints-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(append(body, '\n')); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, s.path)
}

func (s *Store) Load() ([]Hint, error) {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return nil, nil
	}
	body, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var hints []Hint
	if err := json.Unmarshal(body, &hints); err != nil {
		return nil, err
	}
	for _, hint := range hints {
		if err := Validate(hint); err != nil {
			return nil, err
		}
	}
	return hints, nil
}

// Lookup returns the most specific valid hint: exact fingerprint before a
// profile-wide hint, then the one with the latest expiry. Invalid or expired
// hints are ignored rather than allowed to influence a command run.
//
//nolint:maintidx // Selection priority is explicit for auditability.
func (s *Store) Lookup(profile, fingerprint string, now time.Time) (*Hint, error) {
	hints, err := s.Load()
	if err != nil {
		return nil, err
	}
	var candidates []Hint
	for _, hint := range hints {
		if hint.Profile != profile || !hint.ExpiresAt.After(now) {
			continue
		}
		if hint.Fingerprint != "" && hint.Fingerprint != fingerprint {
			continue
		}
		candidates = append(candidates, hint)
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if (candidates[i].Fingerprint != "") != (candidates[j].Fingerprint != "") {
			return candidates[i].Fingerprint != ""
		}
		return candidates[i].ExpiresAt.After(candidates[j].ExpiresAt)
	})
	return &candidates[0], nil
}

func Validate(hint Hint) error {
	if hint.Version != CurrentVersion {
		return errors.New("unsupported budget hint version")
	}
	if strings.TrimSpace(hint.Profile) == "" {
		return errors.New("budget hint profile is required")
	}
	if hint.Direction != DirectionTighten && hint.Direction != DirectionLoosen {
		return errors.New("invalid budget hint direction")
	}
	if hint.Samples <= 0 {
		return errors.New("budget hint samples must be positive")
	}
	if hint.ExpiresAt.IsZero() {
		return errors.New("budget hint expiry is required")
	}
	if hint.Suggested.MaxLines < 0 || hint.Suggested.MaxBytes < 0 || hint.Suggested.MaxTokens < 0 {
		return errors.New("budget hint target cannot be negative")
	}
	return nil
}
