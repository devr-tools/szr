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
	file, err := s.openForAppend(outcome)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := file.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()
	body, err := json.Marshal(outcome)
	if err != nil {
		return err
	}
	_, err = file.Write(append(body, '\n'))
	return err
}

func (s *OutcomeStore) openForAppend(outcome Outcome) (*os.File, error) {
	if !validOutcomeStore(s, outcome) {
		return nil, errors.New("invalid budget hint outcome")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return nil, err
	}
	return os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
}

func validOutcomeStore(store *OutcomeStore, outcome Outcome) bool {
	return store != nil && strings.TrimSpace(store.path) != "" && !outcome.At.IsZero() && outcome.Profile != "" && !outcome.ExpiresAt.IsZero()
}

// ShouldRollback fail-closes a hint after enough local evidence indicates it
// harms fidelity: either fallback or verifier repairs on 20%+ of five runs.
func (s *OutcomeStore) ShouldRollback(hint Hint, now time.Time) bool {
	file, ok := s.openForRollback()
	if !ok {
		return false
	}
	defer file.Close()
	runs, harmful := matchingOutcomeCounts(bufio.NewScanner(file), hint, now)
	return runs >= 5 && harmful*5 >= runs
}

func (s *OutcomeStore) openForRollback() (*os.File, bool) {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return nil, false
	}
	file, err := os.Open(s.path)
	return file, err == nil
}

func matchingOutcomeCounts(scanner *bufio.Scanner, hint Hint, now time.Time) (runs, harmful int) {
	for scanner.Scan() {
		var outcome Outcome
		if json.Unmarshal(scanner.Bytes(), &outcome) != nil || !outcomeMatchesHint(outcome, hint, now) {
			continue
		}
		runs++
		if outcomeIsHarmful(outcome) {
			harmful++
		}
	}
	return runs, harmful
}

func outcomeMatchesHint(outcome Outcome, hint Hint, now time.Time) bool {
	return outcome.ExpiresAt == hint.ExpiresAt && outcome.Profile == hint.Profile && outcome.Fingerprint == hint.Fingerprint && !outcome.At.After(now)
}

func outcomeIsHarmful(outcome Outcome) bool { return outcome.Fallback || outcome.Repair }
