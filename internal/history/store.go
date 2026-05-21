package history

import (
	"bufio"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"sort"
	"strings"
	"time"
)

type Record struct {
	Timestamp          time.Time `json:"timestamp"`
	Command            string    `json:"command"`
	CommandFingerprint string    `json:"command_fingerprint,omitempty"`
	Profile            string    `json:"profile"`
	ProfileConfidence  string    `json:"profile_confidence,omitempty"`
	Cwd                string    `json:"cwd"`
	DurationMS         int64     `json:"duration_ms"`
	ExitCode           int       `json:"exit_code"`
	RawBytes           int       `json:"raw_bytes"`
	FilteredBytes      int       `json:"filtered_bytes"`
	RawBytesRead       int       `json:"raw_bytes_read,omitempty"`
	BytesParsed        int       `json:"bytes_parsed,omitempty"`
	BytesEmitted       int       `json:"bytes_emitted,omitempty"`
	RawTokens          int       `json:"raw_tokens"`
	FilteredTokens     int       `json:"filtered_tokens"`
	SavedTokens        int       `json:"saved_tokens"`
	SavingsPct         float64   `json:"savings_pct"`
	FallbackUsed       bool      `json:"fallback_used,omitempty"`
	TeePath            string    `json:"tee_path,omitempty"`
}

type Store struct {
	path string
}

type Summary struct {
	Commands            int                `json:"commands"`
	AveragePct          float64            `json:"average_pct"`
	SavedTokens         int                `json:"saved_tokens"`
	RawTokens           int                `json:"raw_tokens"`
	FilteredTokens      int                `json:"filtered_tokens"`
	Failures            int                `json:"failures"`
	FailureRate         float64            `json:"failure_rate"`
	Fallbacks           int                `json:"fallbacks"`
	FallbackRate        float64            `json:"fallback_rate"`
	TeeCount            int                `json:"tee_count"`
	TeeRate             float64            `json:"tee_rate"`
	DurationP50MS       int64              `json:"duration_p50_ms"`
	DurationP95MS       int64              `json:"duration_p95_ms"`
	RawBytesRead        int                `json:"raw_bytes_read"`
	BytesParsed         int                `json:"bytes_parsed"`
	BytesEmitted        int                `json:"bytes_emitted"`
	TopCommands         []CommandStat      `json:"top_commands"`
	Recent              []Record           `json:"recent"`
	Profiles            map[string]int     `json:"profiles"`
	ProfileStats        []ProfileStat      `json:"profile_stats"`
	FingerprintHotspots []FingerprintStat  `json:"fingerprint_hotspots"`
	BudgetSuggestions   []BudgetSuggestion `json:"budget_suggestions"`
}

type CommandStat struct {
	Command        string  `json:"command"`
	Count          int     `json:"count"`
	AveragePct     float64 `json:"average_pct"`
	SavedTokens    int     `json:"saved_tokens"`
	RawTokens      int     `json:"raw_tokens"`
	FilteredTokens int     `json:"filtered_tokens"`
}

type ProfileStat struct {
	Name           string  `json:"name"`
	Confidence     string  `json:"confidence,omitempty"`
	Commands       int     `json:"commands"`
	AveragePct     float64 `json:"average_pct"`
	SavedTokens    int     `json:"saved_tokens"`
	RawTokens      int     `json:"raw_tokens"`
	FilteredTokens int     `json:"filtered_tokens"`
	Failures       int     `json:"failures"`
	FailureRate    float64 `json:"failure_rate"`
	Fallbacks      int     `json:"fallbacks"`
	FallbackRate   float64 `json:"fallback_rate"`
	TeeCount       int     `json:"tee_count"`
	TeeRate        float64 `json:"tee_rate"`
	DurationP50MS  int64   `json:"duration_p50_ms"`
	DurationP95MS  int64   `json:"duration_p95_ms"`
}

type FingerprintStat struct {
	Fingerprint   string  `json:"fingerprint"`
	Command       string  `json:"command"`
	Profile       string  `json:"profile"`
	Commands      int     `json:"commands"`
	AveragePct    float64 `json:"average_pct"`
	DurationP50MS int64   `json:"duration_p50_ms"`
	DurationP95MS int64   `json:"duration_p95_ms"`
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

func (s *Store) SuggestBudgets(opts BudgetSuggestionOptions) ([]BudgetSuggestion, error) {
	records, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	return SuggestBudgets(records, opts), nil
}

func Summarize(records []Record, limit int) Summary {
	summary := Summary{
		Profiles: make(map[string]int),
	}
	if len(records) == 0 {
		return summary
	}

	type profileAccumulator struct {
		stat       ProfileStat
		durations  []int64
		confidence map[string]int
	}
	type commandAccumulator struct {
		stat CommandStat
	}
	type fingerprintAccumulator struct {
		stat      FingerprintStat
		durations []int64
	}

	commands := map[string]*commandAccumulator{}
	profileStats := map[string]*profileAccumulator{}
	fingerprintStats := map[string]*fingerprintAccumulator{}
	durations := make([]int64, 0, len(records))

	for _, raw := range records {
		rec := hydrateRecord(raw)
		summary.Commands++
		summary.SavedTokens += rec.SavedTokens
		summary.RawTokens += rec.RawTokens
		summary.FilteredTokens += rec.FilteredTokens
		summary.AveragePct += rec.SavingsPct
		summary.RawBytesRead += rec.RawBytesRead
		summary.BytesParsed += rec.BytesParsed
		summary.BytesEmitted += rec.BytesEmitted
		durations = append(durations, rec.DurationMS)
		if rec.ExitCode != 0 {
			summary.Failures++
		}
		if rec.FallbackUsed {
			summary.Fallbacks++
		}
		if rec.TeePath != "" {
			summary.TeeCount++
		}
		summary.Profiles[rec.Profile]++
		normalizedCommand := normalizeCommand(rec.Command)
		command := commands[normalizedCommand]
		if command == nil {
			command = &commandAccumulator{
				stat: CommandStat{Command: normalizedCommand},
			}
			commands[normalizedCommand] = command
		}
		command.stat.Count++
		command.stat.AveragePct += rec.SavingsPct
		command.stat.SavedTokens += rec.SavedTokens
		command.stat.RawTokens += rec.RawTokens
		command.stat.FilteredTokens += rec.FilteredTokens

		profile := profileStats[rec.Profile]
		if profile == nil {
			profile = &profileAccumulator{
				stat:       ProfileStat{Name: rec.Profile},
				confidence: map[string]int{},
			}
			profileStats[rec.Profile] = profile
		}
		profile.stat.Commands++
		profile.stat.AveragePct += rec.SavingsPct
		profile.stat.SavedTokens += rec.SavedTokens
		profile.stat.RawTokens += rec.RawTokens
		profile.stat.FilteredTokens += rec.FilteredTokens
		if rec.ExitCode != 0 {
			profile.stat.Failures++
		}
		if rec.FallbackUsed {
			profile.stat.Fallbacks++
		}
		if rec.TeePath != "" {
			profile.stat.TeeCount++
		}
		if rec.ProfileConfidence != "" {
			profile.confidence[rec.ProfileConfidence]++
		}
		profile.durations = append(profile.durations, rec.DurationMS)

		fingerprint := fingerprintStats[rec.CommandFingerprint]
		if fingerprint == nil {
			fingerprint = &fingerprintAccumulator{
				stat: FingerprintStat{
					Fingerprint: rec.CommandFingerprint,
					Command:     rec.Command,
					Profile:     rec.Profile,
				},
			}
			fingerprintStats[rec.CommandFingerprint] = fingerprint
		}
		fingerprint.stat.Commands++
		fingerprint.stat.AveragePct += rec.SavingsPct
		fingerprint.durations = append(fingerprint.durations, rec.DurationMS)
	}

	summary.AveragePct /= float64(summary.Commands)
	summary.FailureRate = percent(summary.Failures, summary.Commands)
	summary.FallbackRate = percent(summary.Fallbacks, summary.Commands)
	summary.TeeRate = percent(summary.TeeCount, summary.Commands)
	summary.DurationP50MS = percentile(durations, 50)
	summary.DurationP95MS = percentile(durations, 95)

	for _, command := range commands {
		command.stat.AveragePct /= float64(command.stat.Count)
		summary.TopCommands = append(summary.TopCommands, command.stat)
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
	for i := range recent {
		recent[i] = hydrateRecord(recent[i])
	}
	summary.Recent = recent

	for _, profile := range profileStats {
		profile.stat.AveragePct /= float64(profile.stat.Commands)
		profile.stat.Confidence = dominantConfidence(profile.confidence)
		profile.stat.FailureRate = percent(profile.stat.Failures, profile.stat.Commands)
		profile.stat.FallbackRate = percent(profile.stat.Fallbacks, profile.stat.Commands)
		profile.stat.TeeRate = percent(profile.stat.TeeCount, profile.stat.Commands)
		profile.stat.DurationP50MS = percentile(profile.durations, 50)
		profile.stat.DurationP95MS = percentile(profile.durations, 95)
		summary.ProfileStats = append(summary.ProfileStats, profile.stat)
	}
	sort.Slice(summary.ProfileStats, func(i, j int) bool {
		if summary.ProfileStats[i].Commands == summary.ProfileStats[j].Commands {
			return summary.ProfileStats[i].Name < summary.ProfileStats[j].Name
		}
		return summary.ProfileStats[i].Commands > summary.ProfileStats[j].Commands
	})

	for _, fingerprint := range fingerprintStats {
		fingerprint.stat.AveragePct /= float64(fingerprint.stat.Commands)
		fingerprint.stat.DurationP50MS = percentile(fingerprint.durations, 50)
		fingerprint.stat.DurationP95MS = percentile(fingerprint.durations, 95)
		summary.FingerprintHotspots = append(summary.FingerprintHotspots, fingerprint.stat)
	}
	sort.Slice(summary.FingerprintHotspots, func(i, j int) bool {
		if summary.FingerprintHotspots[i].AveragePct == summary.FingerprintHotspots[j].AveragePct {
			if summary.FingerprintHotspots[i].Commands == summary.FingerprintHotspots[j].Commands {
				return summary.FingerprintHotspots[i].Command < summary.FingerprintHotspots[j].Command
			}
			return summary.FingerprintHotspots[i].Commands > summary.FingerprintHotspots[j].Commands
		}
		return summary.FingerprintHotspots[i].AveragePct < summary.FingerprintHotspots[j].AveragePct
	})
	if len(summary.FingerprintHotspots) > limit {
		summary.FingerprintHotspots = summary.FingerprintHotspots[:limit]
	}
	summary.BudgetSuggestions = SuggestBudgets(records, BudgetSuggestionOptions{Limit: limit})

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

func Fingerprint(command string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(command), " "))
	if normalized == "" {
		return ""
	}
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(normalized))
	return fmt.Sprintf("%016x", hash.Sum64())
}

func hydrateRecord(rec Record) Record {
	if rec.CommandFingerprint == "" {
		rec.CommandFingerprint = Fingerprint(rec.Command)
	}
	if rec.ProfileConfidence == "" {
		rec.ProfileConfidence = inferProfileConfidence(rec.Profile)
	}
	if rec.RawBytesRead == 0 {
		rec.RawBytesRead = rec.RawBytes
		if rec.RawBytesRead == 0 && rec.RawTokens > 0 {
			rec.RawBytesRead = rec.RawTokens * 4
		}
	}
	if rec.BytesParsed == 0 {
		rec.BytesParsed = rec.RawBytesRead
	}
	if rec.BytesEmitted == 0 {
		rec.BytesEmitted = rec.FilteredBytes
		if rec.BytesEmitted == 0 && rec.FilteredTokens > 0 {
			rec.BytesEmitted = rec.FilteredTokens * 4
		}
	}
	if !rec.FallbackUsed && rec.Profile == "passthrough" {
		rec.FallbackUsed = true
	}
	return rec
}

func percent(count, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(count) * 100 / float64(total)
}

func percentile(values []int64, target int) int64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	if target <= 0 {
		return sorted[0]
	}
	if target >= 100 {
		return sorted[len(sorted)-1]
	}
	index := (len(sorted)*target + 99) / 100
	if index <= 0 {
		index = 1
	}
	if index > len(sorted) {
		index = len(sorted)
	}
	return sorted[index-1]
}

func dominantConfidence(counts map[string]int) string {
	best := ""
	bestCount := -1
	for key, count := range counts {
		if count > bestCount || count == bestCount && key < best {
			best = key
			bestCount = count
		}
	}
	return best
}

func inferProfileConfidence(name string) string {
	switch name {
	case "git-status", "git-log", "go-test-json", "vitest-json", "jest-json":
		return "high"
	case "git-diff", "go-build", "generic-test", "js-package-test":
		return "medium"
	case "generic-summary", "passthrough":
		return "low"
	default:
		return ""
	}
}
