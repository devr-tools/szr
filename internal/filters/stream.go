package filters

import (
	"strings"
	"unicode"
)

type ansiStripper struct {
	inEscape bool
	inCSI    bool
}

func (s *ansiStripper) Consume(chunk []byte, emit func(byte)) {
	for _, b := range chunk {
		switch {
		case s.inCSI:
			if b >= 0x40 && b <= 0x7e {
				s.inCSI = false
				s.inEscape = false
			}
		case s.inEscape:
			if b == '[' {
				s.inCSI = true
				continue
			}
			if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') {
				s.inEscape = false
			}
		case b == 0x1b:
			s.inEscape = true
		default:
			emit(b)
		}
	}
}

type lineScanner struct {
	stripper ansiStripper
	pending  strings.Builder
}

func (s *lineScanner) Consume(chunk []byte, emit func(string)) {
	s.stripper.Consume(chunk, func(b byte) {
		switch b {
		case '\r':
			return
		case '\n':
			s.flush(emit)
		default:
			s.pending.WriteByte(b)
		}
	})
}

func (s *lineScanner) Finish(emit func(string)) {
	s.flush(emit)
}

func (s *lineScanner) flush(emit func(string)) {
	if s.pending.Len() == 0 {
		return
	}
	line := strings.TrimRightFunc(s.pending.String(), unicode.IsSpace)
	s.pending.Reset()
	if line != "" {
		emit(line)
	}
}

type LineScanner struct {
	inner lineScanner
}

func (s *LineScanner) Consume(chunk []byte, emit func(string)) {
	s.inner.Consume(chunk, emit)
}

func (s *LineScanner) Finish(emit func(string)) {
	s.inner.Finish(emit)
}

type textBuffer struct {
	stripper ansiStripper
	builder  strings.Builder
}

func (b *textBuffer) Consume(chunk []byte) {
	b.stripper.Consume(chunk, func(c byte) {
		if c != '\r' {
			b.builder.WriteByte(c)
		}
	})
}

func (b *textBuffer) String() string {
	return b.builder.String()
}

func minFilterInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
