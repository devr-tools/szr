package history

import (
	"bufio"
	"encoding/json"
	"os"
)

type Store struct {
	path string
}

func New(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Append(record Record) error {
	file, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	return enc.Encode(record)
}

func (s *Store) LoadAll() ([]Record, error) {
	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var records []Record
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec Record
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		records = append(records, hydrateRecord(rec))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *Store) Clear() error {
	file, err := os.OpenFile(s.path, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	return file.Close()
}

func (s *Store) SuggestBudgets(opts BudgetSuggestionOptions) ([]BudgetSuggestion, error) {
	records, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	return SuggestBudgets(records, opts), nil
}

func (s *Store) FindBudgetSuggestion(fingerprint string, opts BudgetSuggestionOptions) (*BudgetSuggestion, error) {
	records, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	return FindBudgetSuggestion(records, fingerprint, opts), nil
}
