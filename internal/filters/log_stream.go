package filters

import "strings"

// StreamingLogSummarizer aggregates log-shaped text incrementally with
// bounded memory. Chunks stream through the shared line scanner into the
// same severity aggregate the log-aware read reducer uses: distinct-message
// dedup counters capped per severity, with overflow counted rather than
// stored. Nothing buffers the raw stream, so arbitrarily large input keeps a
// fixed footprint. Callers that already know (or were told) the content is
// log-shaped can therefore stream it without the read reducer's sniff window
// and raw-text fallback buffer.
type StreamingLogSummarizer struct {
	scanner  lineScanner
	agg      *logAggregate
	finished bool
}

func NewStreamingLogSummarizer() *StreamingLogSummarizer {
	return &StreamingLogSummarizer{agg: newLogAggregate()}
}

// Consume ingests the next chunk of the stream. Chunks may split lines (and
// ANSI escapes) at arbitrary byte boundaries.
func (s *StreamingLogSummarizer) Consume(chunk []byte) {
	s.scanner.Consume(chunk, s.agg.ingest)
}

// Result flushes any pending partial line and renders the severity summary:
// a histogram line, every distinct ERROR/FATAL message, then lower
// severities by count until maxLines is spent.
func (s *StreamingLogSummarizer) Result(maxLines int) string {
	if !s.finished {
		s.finished = true
		s.scanner.Finish(s.agg.ingest)
	}
	return strings.Join(s.agg.render(maxLines), "\n")
}

// TotalLines reports the number of non-empty lines ingested so far.
func (s *StreamingLogSummarizer) TotalLines() int {
	return s.agg.totalLines
}

// DistinctMessages reports the number of distinct messages seen, including
// ones dropped by the bounded-memory caps.
func (s *StreamingLogSummarizer) DistinctMessages() int {
	return s.agg.distinctTotal()
}
