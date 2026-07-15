package discover

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Transcript lines are JSON objects; assistant entries carry tool_use blocks
// and later user entries carry the matching tool_result content. The parser
// is tolerant to schema drift: unparseable lines and unknown shapes are
// skipped, never fatal.
type transcriptLine struct {
	Type    string            `json:"type"`
	Message transcriptMessage `json:"message"`
}

type transcriptMessage struct {
	Content json.RawMessage `json:"content"`
}

type contentBlock struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
}

type bashInput struct {
	Command string `json:"command"`
}

type commandRun struct {
	Command string
	Output  string
}

func transcriptFiles(dir string, cutoff time.Time) []string {
	var files []string
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			return nil
		}
		if info, infoErr := entry.Info(); infoErr == nil && !info.ModTime().Before(cutoff) {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files
}

// scanTranscriptFile emits one commandRun per Bash tool_use with a matching
// tool_result and returns how many Bash tool_use blocks were seen.
func scanTranscriptFile(path string, emit func(commandRun)) int {
	file, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer file.Close()

	reader := bufio.NewReaderSize(file, 256*1024)
	pending := map[string]string{}
	bashSeen := 0
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			bashSeen += parseTranscriptLine(line, pending, emit)
		}
		if readErr != nil {
			return bashSeen
		}
	}
}

func parseTranscriptLine(line []byte, pending map[string]string, emit func(commandRun)) int {
	var entry transcriptLine
	if err := json.Unmarshal(line, &entry); err != nil {
		return 0
	}
	var blocks []contentBlock
	if len(entry.Message.Content) == 0 || json.Unmarshal(entry.Message.Content, &blocks) != nil {
		return 0
	}
	bashSeen := 0
	for _, block := range blocks {
		switch block.Type {
		case "tool_use":
			bashSeen += collectBashToolUse(block, pending)
		case "tool_result":
			collectToolResult(block, pending, emit)
		}
	}
	return bashSeen
}

func collectBashToolUse(block contentBlock, pending map[string]string) int {
	if block.Name != "Bash" {
		return 0
	}
	var input bashInput
	if json.Unmarshal(block.Input, &input) != nil || block.ID == "" || input.Command == "" {
		return 1
	}
	pending[block.ID] = input.Command
	return 1
}

func collectToolResult(block contentBlock, pending map[string]string, emit func(commandRun)) {
	command, ok := pending[block.ToolUseID]
	if !ok {
		return
	}
	delete(pending, block.ToolUseID)
	emit(commandRun{Command: command, Output: resultText(block.Content)})
}

// resultText handles both tool_result shapes seen in real transcripts: a
// plain string, or a list of {"type":"text","text":...} blocks.
func resultText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var blocks []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}
