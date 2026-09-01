// Package jsonl reads the append-only JSONL stores szr keeps under its data
// directory (command history, tee index, dedup index).
//
// Those records embed the full command text, which has no length bound: one
// `szr grep` over a few hundred paths can produce a line larger than the
// 64KiB default bufio.Scanner token limit. Scanner reports that as a fatal
// read error, so a single oversized record used to break every reader of the
// file permanently - `szr spread` failing with "failed to read history:
// bufio.Scanner: token too long" with no recovery short of deleting the file.
//
// Scan skips oversized lines instead, so one unreadable record costs one
// record. The stores are advisory local analytics, never a source of truth,
// so dropping a record is always preferable to dropping the file.
package jsonl

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"unicode/utf8"
)

// DefaultMaxLineBytes is the longest line Scan parses when the caller does
// not specify a limit.
const DefaultMaxLineBytes = 1 << 20

// readBufferBytes is the initial read buffer. Lines longer than this are
// assembled across several ReadSlice calls.
const readBufferBytes = 64 * 1024

// Scan calls fn for each non-empty line in r and reports how many lines it
// skipped for exceeding maxLineBytes (DefaultMaxLineBytes when non-positive).
// Only genuine I/O errors are returned.
//
// The slice handed to fn is reused between lines: callers that keep it must
// copy first.
func Scan(r io.Reader, maxLineBytes int, fn func(line []byte)) (int, error) {
	if maxLineBytes <= 0 {
		maxLineBytes = DefaultMaxLineBytes
	}
	lines := &lineReader{
		reader: bufio.NewReaderSize(r, readBufferBytes),
		max:    maxLineBytes,
		line:   make([]byte, 0, readBufferBytes),
	}
	for {
		line, done, err := lines.next()
		if err != nil {
			return lines.skipped, err
		}
		if len(line) > 0 {
			fn(line)
		}
		if done {
			return lines.skipped, nil
		}
	}
}

// lineReader assembles lines that span several reads while discarding any
// line longer than max.
type lineReader struct {
	reader    *bufio.Reader
	max       int
	line      []byte
	oversized bool
	skipped   int
}

// next returns the next line, empty for a blank or dropped line, along with
// done once the reader is exhausted. The returned slice is valid until the
// following call.
func (l *lineReader) next() ([]byte, bool, error) {
	l.line = l.line[:0]
	l.oversized = false
	for {
		chunk, err := l.reader.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			l.add(chunk)
			continue
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, false, err
		}
		l.add(bytes.TrimRight(chunk, "\r\n"))
		done := errors.Is(err, io.EOF)
		if l.oversized {
			l.skipped++
			return nil, done, nil
		}
		return l.line, done, nil
	}
}

// add appends chunk unless doing so would push the line past max, in which
// case the line is abandoned and the rest of it discarded.
func (l *lineReader) add(chunk []byte) {
	if l.oversized {
		return
	}
	if len(l.line)+len(chunk) > l.max {
		l.oversized = true
		l.line = l.line[:0]
		return
	}
	l.line = append(l.line, chunk...)
}

// Clip bounds a record field at a rune boundary so writers cannot produce a
// line that Scan would later have to drop. Callers apply it to the free-form
// command text, the only unbounded field these records carry.
func Clip(text string, maxBytes int) string {
	if maxBytes <= 0 || len(text) <= maxBytes {
		return text
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut] + "\u2026"
}
