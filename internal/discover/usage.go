package discover

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const DefaultUsageSinceDays = 7

// UsageOptions controls a model-usage scan over agent transcripts.
type UsageOptions struct {
	// Root is the transcripts root, e.g. ~/.claude/projects.
	Root string
	// Project restricts the scan to one encoded project dir; empty scans all.
	Project string
	Since   time.Duration
	Now     time.Time
	// SessionPrefix keeps only sessions whose id starts with the prefix.
	SessionPrefix string
}

// SessionUsage aggregates the model-reported usage counters of one agent
// session, including subagent transcripts stored under the session's
// subdirectory. Token counts come straight from transcript usage objects and
// are exact as recorded by the agent runtime.
type SessionUsage struct {
	SessionID           string    `json:"session_id"`
	Project             string    `json:"project"`
	Cwd                 string    `json:"cwd,omitempty"`
	FirstSeen           time.Time `json:"first_seen"`
	LastSeen            time.Time `json:"last_seen"`
	Turns               int       `json:"turns"`
	InputTokens         int       `json:"input_tokens"`
	CacheCreationTokens int       `json:"cache_creation_tokens"`
	CacheReadTokens     int       `json:"cache_read_tokens"`
	OutputTokens        int       `json:"output_tokens"`
	TranscriptFiles     int       `json:"transcript_files"`
	// Main covers the session's own transcript only; Agents covers subagent
	// transcripts with billed usage. Session totals equal Main plus all Agents.
	Main   AgentUsage   `json:"main"`
	Agents []AgentUsage `json:"agents,omitempty"`
}

// FreshInputTokens counts input the model processed anew: direct input plus
// cache-creation input, excluding cache reads.
func (s SessionUsage) FreshInputTokens() int {
	return s.InputTokens + s.CacheCreationTokens
}

// AgentUsage aggregates the usage counters of one transcript file: a subagent
// transcript, or the session's main transcript when AgentID is empty.
type AgentUsage struct {
	AgentID             string `json:"agent_id,omitempty"`
	Turns               int    `json:"turns"`
	InputTokens         int    `json:"input_tokens"`
	CacheCreationTokens int    `json:"cache_creation_tokens"`
	CacheReadTokens     int    `json:"cache_read_tokens"`
	OutputTokens        int    `json:"output_tokens"`
}

// FreshInputTokens counts input the model processed anew: direct input plus
// cache-creation input, excluding cache reads.
func (a AgentUsage) FreshInputTokens() int {
	return a.InputTokens + a.CacheCreationTokens
}

type usageLine struct {
	Type      string       `json:"type"`
	Timestamp string       `json:"timestamp"`
	Cwd       string       `json:"cwd"`
	UUID      string       `json:"uuid"`
	Message   usageMessage `json:"message"`
}

type usageMessage struct {
	ID    string       `json:"id"`
	Usage *usageCounts `json:"usage"`
}

type usageCounts struct {
	InputTokens              int `json:"input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	OutputTokens             int `json:"output_tokens"`
}

type sessionAccum struct {
	usage SessionUsage
	// transcripts accumulates per transcript file, keyed by agent id with ""
	// for the session's main transcript.
	transcripts map[string]*transcriptAccum
}

type transcriptAccum struct {
	agentID string
	// turns keeps the last usage payload seen per message id: streamed
	// transcripts repeat one message across several lines with cumulative
	// counters, so only the final payload for an id is billed once. The map
	// is scoped to one transcript file.
	turns     map[string]usageCounts
	synthetic int
}

func (a *sessionAccum) transcript(agentID string) *transcriptAccum {
	accum, ok := a.transcripts[agentID]
	if !ok {
		accum = &transcriptAccum{agentID: agentID, turns: map[string]usageCounts{}}
		a.transcripts[agentID] = accum
	}
	return accum
}

// ScanUsage aggregates model-reported token usage per session under Root.
// Subagent transcripts stored in a session's subdirectory are attributed to
// that session. Sessions without any usage payloads are omitted.
func ScanUsage(opts UsageOptions) []SessionUsage {
	opts = usageDefaults(opts)
	cutoff := opts.Now.Add(-opts.Since)
	sessions := map[string]*sessionAccum{}
	for _, dir := range projectDirs(Options{Root: opts.Root, Project: opts.Project}) {
		for _, file := range transcriptFiles(dir, cutoff) {
			session := usageSession(sessions, dir, file)
			scanUsageFile(file, session, session.transcript(usageAgentID(dir, file)))
		}
	}
	return finalizeUsage(sessions, opts.SessionPrefix)
}

func usageDefaults(opts UsageOptions) UsageOptions {
	if opts.Since <= 0 {
		opts.Since = DefaultUsageSinceDays * 24 * time.Hour
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	return opts
}

func usageSession(sessions map[string]*sessionAccum, dir, file string) *sessionAccum {
	project := filepath.Base(dir)
	id := sessionIDForPath(dir, file)
	key := project + "/" + id
	accum, ok := sessions[key]
	if !ok {
		accum = &sessionAccum{
			usage:       SessionUsage{SessionID: id, Project: project},
			transcripts: map[string]*transcriptAccum{},
		}
		sessions[key] = accum
	}
	accum.usage.TranscriptFiles++
	return accum
}

// sessionIDForPath maps a transcript file to its session: top-level files are
// sessions themselves; nested files (subagent transcripts) belong to the
// session subdirectory that contains them.
func sessionIDForPath(dir, file string) string {
	rel, err := filepath.Rel(dir, file)
	if err != nil {
		return strings.TrimSuffix(filepath.Base(file), ".jsonl")
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) == 1 {
		return strings.TrimSuffix(parts[0], ".jsonl")
	}
	return parts[0]
}

// usageAgentID maps a transcript file to its subagent id, derived from the
// agent-*.jsonl filename; the session's main transcript maps to "".
func usageAgentID(dir, file string) string {
	rel, err := filepath.Rel(dir, file)
	if err != nil || !strings.Contains(rel, string(filepath.Separator)) {
		return ""
	}
	base := strings.TrimSuffix(filepath.Base(file), ".jsonl")
	if id := strings.TrimPrefix(base, "agent-"); id != "" {
		return id
	}
	return base
}

func scanUsageFile(path string, session *sessionAccum, transcript *transcriptAccum) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	reader := bufio.NewReaderSize(file, 256*1024)
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			parseUsageLine(line, session, transcript)
		}
		if readErr != nil {
			return
		}
	}
}

func parseUsageLine(line []byte, session *sessionAccum, transcript *transcriptAccum) {
	var entry usageLine
	if err := json.Unmarshal(line, &entry); err != nil {
		return
	}
	recordUsageTimestamp(session, entry.Timestamp)
	if session.usage.Cwd == "" {
		session.usage.Cwd = entry.Cwd
	}
	if entry.Type != "assistant" || entry.Message.Usage == nil {
		return
	}
	transcript.turns[usageTurnKey(transcript, entry)] = *entry.Message.Usage
}

func recordUsageTimestamp(accum *sessionAccum, raw string) {
	if raw == "" {
		return
	}
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return
	}
	if accum.usage.FirstSeen.IsZero() || ts.Before(accum.usage.FirstSeen) {
		accum.usage.FirstSeen = ts
	}
	if ts.After(accum.usage.LastSeen) {
		accum.usage.LastSeen = ts
	}
}

func usageTurnKey(accum *transcriptAccum, entry usageLine) string {
	if entry.Message.ID != "" {
		return entry.Message.ID
	}
	if entry.UUID != "" {
		return "uuid:" + entry.UUID
	}
	accum.synthetic++
	return fmt.Sprintf("synthetic:%d", accum.synthetic)
}

func finalizeUsage(sessions map[string]*sessionAccum, prefix string) []SessionUsage {
	out := make([]SessionUsage, 0, len(sessions))
	for _, accum := range sessions {
		usage := sumSessionUsage(accum)
		if usage.Turns == 0 || !strings.HasPrefix(usage.SessionID, prefix) {
			continue
		}
		out = append(out, usage)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].LastSeen.Equal(out[j].LastSeen) {
			return out[i].LastSeen.After(out[j].LastSeen)
		}
		return out[i].SessionID < out[j].SessionID
	})
	return out
}

// sumSessionUsage folds every transcript into the session totals while keeping
// the main transcript and each billed subagent distinguishable, so that the
// totals always equal Main plus the sum of Agents.
func sumSessionUsage(accum *sessionAccum) SessionUsage {
	usage := accum.usage
	for id, transcript := range accum.transcripts {
		part := sumTranscriptTurns(transcript)
		addAgentUsage(&usage, part)
		switch {
		case id == "":
			usage.Main = part
		case part.Turns > 0:
			usage.Agents = append(usage.Agents, part)
		}
	}
	sort.Slice(usage.Agents, func(i, j int) bool {
		return usage.Agents[i].AgentID < usage.Agents[j].AgentID
	})
	return usage
}

func sumTranscriptTurns(accum *transcriptAccum) AgentUsage {
	part := AgentUsage{AgentID: accum.agentID, Turns: len(accum.turns)}
	for _, counts := range accum.turns {
		part.InputTokens += counts.InputTokens
		part.CacheCreationTokens += counts.CacheCreationInputTokens
		part.CacheReadTokens += counts.CacheReadInputTokens
		part.OutputTokens += counts.OutputTokens
	}
	return part
}

func addAgentUsage(usage *SessionUsage, part AgentUsage) {
	usage.Turns += part.Turns
	usage.InputTokens += part.InputTokens
	usage.CacheCreationTokens += part.CacheCreationTokens
	usage.CacheReadTokens += part.CacheReadTokens
	usage.OutputTokens += part.OutputTokens
}
