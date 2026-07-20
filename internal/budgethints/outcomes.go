package budgethints

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Outcome is local-only aggregate evidence about an applied gateway hint. It
// never includes command text, output, paths, or provider data.
type Outcome struct {
	At          time.Time `json:"at"`
	Profile     string    `json:"profile"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	ExpiresAt   time.Time `json:"expires_at"`
	Fallback    bool      `json:"fallback"`
	Repair      bool      `json:"repair"`
}

type OutcomeStore struct{ path string }

func NewOutcomeStore(path string) *OutcomeStore { return &OutcomeStore{path: path} }

func (s *OutcomeStore) Append(outcome Outcome) (err error) {
	if s == nil || strings.TrimSpace(s.path) == "" || outcome.At.IsZero() || outcome.Profile == "" || outcome.ExpiresAt.IsZero() {
		return errors.New("invalid budget hint outcome")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	body, err := json.Marshal(outcome)
	if err != nil {
		return err
	}
	_, err = file.Write(append(body, '\n'))
	return err
}

// ShouldRollback fail-closes a hint after enough local evidence indicates it
// harms fidelity: either fallback or verifier repairs on 20%+ of five runs.
func (s *OutcomeStore) ShouldRollback(hint Hint, now time.Time) bool {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return false
	}
	file, err := os.Open(s.path)
	if err != nil {
		return false
	}
	defer file.Close()
	var runs, harmful int
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var outcome Outcome
		if json.Unmarshal(scanner.Bytes(), &outcome) != nil || outcome.ExpiresAt != hint.ExpiresAt || outcome.Profile != hint.Profile || outcome.Fingerprint != hint.Fingerprint || outcome.At.After(now) {
			continue
		}
		runs++
		if outcome.Fallback || outcome.Repair {
			harmful++
		}
	}
	return runs >= 5 && harmful*5 >= runs
}
