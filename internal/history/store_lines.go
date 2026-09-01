package history

import (
	"bufio"
	"encoding/json"
	"os"

	"github.com/devr-tools/szr/internal/jsonl"
)

// repairMaxLineBytes is how far a read will go to recover a single oversized
// line. A line that long is normally a perfectly good record carrying a huge
// command string, so re-encoding it with the command clipped keeps its
// measurements instead of discarding a real run.
const repairMaxLineBytes = 8 << 20

// historyLine pairs a decoded record with the bytes to write back for it.
type historyLine struct {
	data     []byte
	record   Record
	repaired bool
}

// lineStats reports what a read had to change or discard.
type lineStats struct {
	// repaired counts oversized records re-encoded with a clipped command.
	repaired int
	// dropped counts lines no reader can parse: torn writes from concurrent
	// appends, and lines too large to recover at all.
	dropped int
}

// decodeRecord parses one history line. oversized reports a parseable record
// whose line exceeds maxRecordLineBytes; its command is clipped so the record
// fits what readers parse, and compaction persists the clipped form.
func decodeRecord(line []byte) (rec Record, oversized bool, ok bool) {
	if err := json.Unmarshal(line, &rec); err != nil {
		return Record{}, false, false
	}
	if len(line) <= maxRecordLineBytes {
		return rec, false, true
	}
	rec.Command = jsonl.Clip(rec.Command, maxCommandBytes)
	return rec, true, true
}

// readHistoryLines reads the history file into decoded lines, repairing
// oversized records and discarding unparseable ones. It returns ok=false on
// open or read errors.
func (s *Store) readHistoryLines() ([]historyLine, lineStats, bool) {
	file, err := os.Open(s.path)
	if err != nil {
		return nil, lineStats{}, false
	}
	defer file.Close()

	var acc lineAccumulator
	unreadable, err := jsonl.Scan(file, repairMaxLineBytes, acc.add)
	if err != nil {
		return nil, lineStats{}, false
	}
	acc.stats.dropped += unreadable
	return acc.lines, acc.stats, true
}

// lineAccumulator collects decoded lines and what had to change to get them.
type lineAccumulator struct {
	lines []historyLine
	stats lineStats
}

func (a *lineAccumulator) add(line []byte) {
	rec, oversized, ok := decodeRecord(line)
	if !ok {
		a.stats.dropped++
		return
	}
	data, ok := lineBytes(line, rec, oversized)
	if !ok {
		a.stats.dropped++
		return
	}
	if oversized {
		a.stats.repaired++
	}
	a.lines = append(a.lines, historyLine{data: data, record: hydrateRecord(rec), repaired: oversized})
}

// lineBytes returns what should be written back for a decoded line: the
// original bytes, or a re-encoded line for a repaired record.
func lineBytes(line []byte, rec Record, oversized bool) ([]byte, bool) {
	if !oversized {
		return append([]byte(nil), line...), true
	}
	clipped, err := json.Marshal(rec)
	if err != nil {
		return nil, false
	}
	return clipped, true
}

// retainStart walks backward from the newest line and returns the index of
// the oldest line to keep. The newest line is always kept; older lines are
// added until the record cap is reached or the byte budget (line length plus
// trailing newline) would be exceeded.
func retainStart(lines []historyLine, retainRecords int, retainBytes int64) int {
	total := int64(0)
	start := len(lines)
	for start > 0 && len(lines)-start < retainRecords {
		lineBytes := int64(len(lines[start-1].data) + 1)
		if start < len(lines) && total+lineBytes > retainBytes {
			break
		}
		total += lineBytes
		start--
	}
	return start
}

func writeCompactedLines(file *os.File, lines []historyLine) bool {
	writer := bufio.NewWriter(file)
	for _, line := range lines {
		if _, err := writer.Write(line.data); err != nil {
			_ = file.Close()
			return false
		}
		if err := writer.WriteByte('\n'); err != nil {
			_ = file.Close()
			return false
		}
	}
	if err := writer.Flush(); err != nil {
		_ = file.Close()
		return false
	}
	return file.Close() == nil
}
