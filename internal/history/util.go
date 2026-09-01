package history

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
)

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
