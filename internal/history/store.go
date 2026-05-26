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
	CommandHotspots     []CommandHotspot   `json:"command_hotspots"`
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

type CommandHotspot struct {
	Command       string  `json:"command"`
	Profile       string  `json:"profile"`
	Commands      int     `json:"commands"`
	AveragePct    float64 `json:"average_pct"`
	FailureRate   float64 `json:"failure_rate"`
	FallbackRate  float64 `json:"fallback_rate"`
	DurationP50MS int64   `json:"duration_p50_ms"`
	DurationP95MS int64   `json:"duration_p95_ms"`
}

type summaryProfileAccumulator struct {
	stat       ProfileStat
	durations  []int64
	confidence map[string]int
}

type summaryCommandAccumulator struct {
	stat CommandStat
}

type summaryFingerprintAccumulator struct {
	stat      FingerprintStat
	durations []int64
	rawTokens int
	filtered  int
}

type summaryCommandHotspotAccumulator struct {
	stat      CommandHotspot
	failures  int
	fallbacks int
	durations []int64
	rawTokens int
	filtered  int
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

	commands := map[string]*summaryCommandAccumulator{}
	profileStats := map[string]*summaryProfileAccumulator{}
	commandHotspots := map[string]*summaryCommandHotspotAccumulator{}
	fingerprintStats := map[string]*summaryFingerprintAccumulator{}
	durations := make([]int64, 0, len(records))

	for _, raw := range records {
		rec := hydrateRecord(raw)
		updateSummaryTotals(&summary, rec)
		durations = append(durations, rec.DurationMS)
		updateSummaryCommand(commands, rec)
		updateSummaryProfile(profileStats, rec)
		updateSummaryCommandHotspot(commandHotspots, rec)
		updateSummaryFingerprint(fingerprintStats, rec)
	}

	summary.AveragePct /= float64(summary.Commands)
	summary.FailureRate = percent(summary.Failures, summary.Commands)
	summary.FallbackRate = percent(summary.Fallbacks, summary.Commands)
	summary.TeeRate = percent(summary.TeeCount, summary.Commands)
	summary.DurationP50MS = percentile(durations, 50)
	summary.DurationP95MS = percentile(durations, 95)
	summary.TopCommands = summarizeTopCommands(commands, limit)
	summary.Recent = summarizeRecent(records, limit)
	summary.ProfileStats = summarizeProfiles(profileStats)
	summary.CommandHotspots = summarizeCommandHotspots(commandHotspots, limit)
	summary.FingerprintHotspots = summarizeFingerprints(fingerprintStats, limit)
	summary.BudgetSuggestions = SuggestBudgets(records, BudgetSuggestionOptions{Limit: limit})

	return summary
}

func updateSummaryTotals(summary *Summary, rec Record) {
	summary.Commands++
	summary.SavedTokens += rec.SavedTokens
	summary.RawTokens += rec.RawTokens
	summary.FilteredTokens += rec.FilteredTokens
	summary.AveragePct += rec.SavingsPct
	summary.RawBytesRead += rec.RawBytesRead
	summary.BytesParsed += rec.BytesParsed
	summary.BytesEmitted += rec.BytesEmitted
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
}

func updateSummaryCommand(commands map[string]*summaryCommandAccumulator, rec Record) {
	normalizedCommand := normalizeCommand(rec.Command)
	command := commands[normalizedCommand]
	if command == nil {
		command = &summaryCommandAccumulator{stat: CommandStat{Command: normalizedCommand}}
		commands[normalizedCommand] = command
	}
	command.stat.Count++
	command.stat.AveragePct += rec.SavingsPct
	command.stat.SavedTokens += rec.SavedTokens
	command.stat.RawTokens += rec.RawTokens
	command.stat.FilteredTokens += rec.FilteredTokens
}

func updateSummaryProfile(profileStats map[string]*summaryProfileAccumulator, rec Record) {
	profile := profileStats[rec.Profile]
	if profile == nil {
		profile = &summaryProfileAccumulator{
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
}

func updateSummaryCommandHotspot(commandHotspots map[string]*summaryCommandHotspotAccumulator, rec Record) {
	normalizedCommand := normalizeCommand(rec.Command)
	key := normalizedCommand + "\x00" + rec.Profile
	hotspot := commandHotspots[key]
	if hotspot == nil {
		hotspot = &summaryCommandHotspotAccumulator{
			stat: CommandHotspot{
				Command: normalizedCommand,
				Profile: rec.Profile,
			},
		}
		commandHotspots[key] = hotspot
	}
	hotspot.stat.Commands++
	hotspot.stat.AveragePct += rec.SavingsPct
	if rec.ExitCode != 0 {
		hotspot.failures++
	}
	if rec.FallbackUsed {
		hotspot.fallbacks++
	}
	hotspot.durations = append(hotspot.durations, rec.DurationMS)
	hotspot.rawTokens += rec.RawTokens
	hotspot.filtered += rec.FilteredTokens
}

func updateSummaryFingerprint(fingerprints map[string]*summaryFingerprintAccumulator, rec Record) {
	fingerprint := fingerprints[rec.CommandFingerprint]
	if fingerprint == nil {
		fingerprint = &summaryFingerprintAccumulator{
			stat: FingerprintStat{
				Fingerprint: rec.CommandFingerprint,
				Command:     rec.Command,
				Profile:     rec.Profile,
			},
		}
		fingerprints[rec.CommandFingerprint] = fingerprint
	}
	fingerprint.stat.Commands++
	fingerprint.stat.AveragePct += rec.SavingsPct
	fingerprint.durations = append(fingerprint.durations, rec.DurationMS)
	fingerprint.rawTokens += rec.RawTokens
	fingerprint.filtered += rec.FilteredTokens
}

func summarizeTopCommands(commands map[string]*summaryCommandAccumulator, limit int) []CommandStat {
	list := make([]CommandStat, 0, len(commands))
	for _, command := range commands {
		command.stat.AveragePct /= float64(command.stat.Count)
		list = append(list, command.stat)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Count == list[j].Count {
			return list[i].Command < list[j].Command
		}
		return list[i].Count > list[j].Count
	})
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	return list
}

func summarizeRecent(records []Record, limit int) []Record {
	recent := append([]Record(nil), records...)
	sort.Slice(recent, func(i, j int) bool {
		return recent[i].Timestamp.After(recent[j].Timestamp)
	})
	if limit > 0 && len(recent) > limit {
		recent = recent[:limit]
	}
	for i := range recent {
		recent[i] = hydrateRecord(recent[i])
	}
	return recent
}

func summarizeProfiles(profileStats map[string]*summaryProfileAccumulator) []ProfileStat {
	list := make([]ProfileStat, 0, len(profileStats))
	for _, profile := range profileStats {
		profile.stat.AveragePct /= float64(profile.stat.Commands)
		profile.stat.Confidence = dominantConfidence(profile.confidence)
		profile.stat.FailureRate = percent(profile.stat.Failures, profile.stat.Commands)
		profile.stat.FallbackRate = percent(profile.stat.Fallbacks, profile.stat.Commands)
		profile.stat.TeeRate = percent(profile.stat.TeeCount, profile.stat.Commands)
		profile.stat.DurationP50MS = percentile(profile.durations, 50)
		profile.stat.DurationP95MS = percentile(profile.durations, 95)
		list = append(list, profile.stat)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Commands == list[j].Commands {
			return list[i].Name < list[j].Name
		}
		return list[i].Commands > list[j].Commands
	})
	return list
}

func summarizeCommandHotspots(commandHotspots map[string]*summaryCommandHotspotAccumulator, limit int) []CommandHotspot {
	type scoredHotspot struct {
		stat     CommandHotspot
		severity float64
	}
	scored := make([]scoredHotspot, 0, len(commandHotspots))
	for _, hotspot := range commandHotspots {
		hotspot.stat.AveragePct /= float64(hotspot.stat.Commands)
		hotspot.stat.FailureRate = percent(hotspot.failures, hotspot.stat.Commands)
		hotspot.stat.FallbackRate = percent(hotspot.fallbacks, hotspot.stat.Commands)
		hotspot.stat.DurationP50MS = percentile(hotspot.durations, 50)
		hotspot.stat.DurationP95MS = percentile(hotspot.durations, 95)
		severity := commandHotspotSeverity(hotspot.stat, hotspot.rawTokens, hotspot.filtered)
		if hotspot.stat.Commands < 2 &&
			hotspot.rawTokens < 48 &&
			hotspot.stat.AveragePct >= 0 &&
			hotspot.stat.FallbackRate == 0 &&
			hotspot.stat.FailureRate == 0 {
			continue
		}
		scored = append(scored, scoredHotspot{stat: hotspot.stat, severity: severity})
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].severity == scored[j].severity {
			if scored[i].stat.Commands == scored[j].stat.Commands {
				return scored[i].stat.Command < scored[j].stat.Command
			}
			return scored[i].stat.Commands > scored[j].stat.Commands
		}
		return scored[i].severity > scored[j].severity
	})
	if limit > 0 && len(scored) > limit {
		scored = scored[:limit]
	}
	list := make([]CommandHotspot, 0, len(scored))
	for _, item := range scored {
		list = append(list, item.stat)
	}
	return list
}

func summarizeFingerprints(fingerprintStats map[string]*summaryFingerprintAccumulator, limit int) []FingerprintStat {
	type scoredFingerprint struct {
		stat     FingerprintStat
		severity float64
	}
	scored := make([]scoredFingerprint, 0, len(fingerprintStats))
	for _, fingerprint := range fingerprintStats {
		fingerprint.stat.AveragePct /= float64(fingerprint.stat.Commands)
		fingerprint.stat.DurationP50MS = percentile(fingerprint.durations, 50)
		fingerprint.stat.DurationP95MS = percentile(fingerprint.durations, 95)
		if fingerprint.stat.Commands < 2 && fingerprint.stat.AveragePct >= 0 {
			continue
		}
		if fingerprint.stat.Commands < 2 && fingerprint.rawTokens < 16 {
			continue
		}
		if fingerprint.stat.Commands < 2 && fingerprint.rawTokens < 24 && fingerprint.stat.AveragePct > -25 {
			continue
		}
		scored = append(scored, scoredFingerprint{
			stat:     fingerprint.stat,
			severity: fingerprintHotspotSeverity(fingerprint.stat, fingerprint.rawTokens, fingerprint.filtered),
		})
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].severity == scored[j].severity {
			if scored[i].stat.AveragePct == scored[j].stat.AveragePct {
				if scored[i].stat.Commands == scored[j].stat.Commands {
					return scored[i].stat.Command < scored[j].stat.Command
				}
				return scored[i].stat.Commands > scored[j].stat.Commands
			}
			return scored[i].stat.AveragePct < scored[j].stat.AveragePct
		}
		return scored[i].severity > scored[j].severity
	})
	if limit > 0 && len(scored) > limit {
		scored = scored[:limit]
	}
	list := make([]FingerprintStat, 0, len(scored))
	for _, item := range scored {
		list = append(list, item.stat)
	}
	return list
}

func commandHotspotSeverity(item CommandHotspot, rawTokens, filteredTokens int) float64 {
	volumeWeight := scaledVolumeWeight(rawTokens, 64)
	emitted := float64(filteredTokens) * volumeWeight
	fallbackPenalty := float64(rawTokens) * (item.FallbackRate / 100) * 0.35
	failurePenalty := float64(rawTokens) * (item.FailureRate / 100) * 0.20
	latencyPenalty := float64(item.DurationP95MS) * volumeWeight / 10
	efficiencyPenalty := maxFloat(0, 12-item.AveragePct) * float64(rawTokens) / 100
	return emitted + fallbackPenalty + failurePenalty + latencyPenalty + efficiencyPenalty
}

func fingerprintHotspotSeverity(item FingerprintStat, rawTokens, filteredTokens int) float64 {
	volumeWeight := scaledVolumeWeight(rawTokens, 32)
	overhead := 0.0
	if item.AveragePct < 0 {
		overhead = -item.AveragePct * float64(rawTokens) / 100 * 2
	}
	poorSavings := maxFloat(0, 10-item.AveragePct) * float64(rawTokens) / 100
	return (float64(filteredTokens) + overhead + poorSavings) * volumeWeight
}

func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}

	byteEstimate := (len(text) + 3) / 4
	wordRuns := 0
	symbolRuns := 0
	extraWordSplits := 0
	currentKind := byteClassSpace
	currentLen := 0

	flush := func() {
		if currentKind == byteClassWord {
			wordRuns++
			if currentLen > 12 {
				extraWordSplits += (currentLen - 9) / 8
			}
		} else if currentKind == byteClassSymbol {
			symbolRuns++
		}
	}

	for i := 0; i < len(text); i++ {
		kind := classifyTokenByte(text[i])
		if kind == currentKind {
			currentLen++
			continue
		}
		if currentLen > 0 {
			flush()
		}
		currentKind = kind
		if kind == byteClassSpace {
			currentLen = 0
			continue
		}
		currentLen = 1
	}
	if currentLen > 0 {
		flush()
	}

	lexicalEstimate := wordRuns + extraWordSplits + (symbolRuns+1)/2
	if lexicalEstimate < 1 {
		lexicalEstimate = 1
	}
	if lexicalEstimate > byteEstimate {
		return lexicalEstimate
	}
	return byteEstimate
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

const (
	byteClassSpace = iota
	byteClassWord
	byteClassSymbol
)

func classifyTokenByte(b byte) int {
	switch {
	case b == ' ' || b == '\n' || b == '\r' || b == '\t' || b == '\f' || b == '\v':
		return byteClassSpace
	case (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9'):
		return byteClassWord
	default:
		return byteClassSymbol
	}
}

func scaledVolumeWeight(tokens, floor int) float64 {
	if tokens <= 0 {
		return 0
	}
	return float64(tokens) / float64(tokens+floor)
}

func maxFloat(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}
