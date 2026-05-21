package history

import (
	"bufio"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"time"
)

type Record struct {
	Timestamp      time.Time `json:"timestamp"`
	Command        string    `json:"command"`
	Profile        string    `json:"profile"`
	Cwd            string    `json:"cwd"`
	DurationMS     int64     `json:"duration_ms"`
	ExitCode       int       `json:"exit_code"`
	RawBytes       int       `json:"raw_bytes"`
	FilteredBytes  int       `json:"filtered_bytes"`
	RawTokens      int       `json:"raw_tokens"`
	FilteredTokens int       `json:"filtered_tokens"`
	SavedTokens    int       `json:"saved_tokens"`
	SavingsPct     float64   `json:"savings_pct"`
	TeePath        string    `json:"tee_path,omitempty"`
}

type Store struct {
	path string
}

type Summary struct {
	Commands      int            `json:"commands"`
	AveragePct    float64        `json:"average_pct"`
	SavedTokens   int            `json:"saved_tokens"`
	RawTokens     int            `json:"raw_tokens"`
	FilteredToken int            `json:"filtered_tokens"`
	Failures      int            `json:"failures"`
	TopCommands   []CommandStat  `json:"top_commands"`
	Recent        []Record       `json:"recent"`
	Profiles      map[string]int `json:"profiles"`
}

type CommandStat struct {
	Command string `json:"command"`
	Count   int    `json:"count"`
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
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func Summarize(records []Record, limit int) Summary {
	summary := Summary{
		Profiles: make(map[string]int),
	}
	if len(records) == 0 {
		return summary
	}

	commands := map[string]int{}
	for _, rec := range records {
		summary.Commands++
		summary.SavedTokens += rec.SavedTokens
		summary.RawTokens += rec.RawTokens
		summary.FilteredToken += rec.FilteredTokens
		summary.AveragePct += rec.SavingsPct
		if rec.ExitCode != 0 {
			summary.Failures++
		}
		summary.Profiles[rec.Profile]++
		commands[normalizeCommand(rec.Command)]++
	}
	summary.AveragePct /= float64(summary.Commands)

	for cmd, count := range commands {
		summary.TopCommands = append(summary.TopCommands, CommandStat{Command: cmd, Count: count})
	}
	sort.Slice(summary.TopCommands, func(i, j int) bool {
		if summary.TopCommands[i].Count == summary.TopCommands[j].Count {
			return summary.TopCommands[i].Command < summary.TopCommands[j].Command
		}
		return summary.TopCommands[i].Count > summary.TopCommands[j].Count
	})
	if len(summary.TopCommands) > limit {
		summary.TopCommands = summary.TopCommands[:limit]
	}

	recent := append([]Record(nil), records...)
	sort.Slice(recent, func(i, j int) bool {
		return recent[i].Timestamp.After(recent[j].Timestamp)
	})
	if len(recent) > limit {
		recent = recent[:limit]
	}
	summary.Recent = recent

	return summary
}

func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	runes := len([]rune(text))
	if runes < 4 {
		return 1
	}
	return (runes + 3) / 4
}

func normalizeCommand(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return ""
	}
	if len(fields) > 3 {
		fields = fields[:3]
	}
	return strings.Join(fields, " ")
}
