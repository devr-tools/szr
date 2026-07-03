package filters

import (
	"strings"
	"unicode"
)

type ansiStripper struct {
	inEscape  bool // saw ESC, deciding what kind of sequence follows
	inCSI     bool // inside ESC [ ... ; ends on a final byte 0x40-0x7e
	inString  bool // inside OSC/DCS/SOS/PM/APC; ends on BEL or ST (ESC \)
	stringEsc bool // saw ESC inside a string sequence, checking for ST
}

func (s *ansiStripper) Consume(chunk []byte, emit func(byte)) {
	for _, b := range chunk {
		switch {
		case s.stringEsc:
			// ESC seen inside an OSC/DCS-style string: ESC \ (ST) ends it.
			s.stringEsc = false
			if b == '\\' {
				s.inString = false
			} else if b == 0x1b {
				s.stringEsc = true
			}
		case s.inString:
			if b == 0x07 { // BEL terminator (common for OSC)
				s.inString = false
			} else if b == 0x1b {
				s.stringEsc = true
			}
		case s.inCSI:
			// Parameter (0x30-0x3f) and intermediate (0x20-0x2f) bytes are
			// consumed; a final byte 0x40-0x7e ends the sequence.
			if b >= 0x40 && b <= 0x7e {
				s.inCSI = false
			}
		case s.inEscape:
			s.inEscape = false
			switch {
			case b == '[':
				s.inCSI = true
			case b == ']' || b == 'P' || b == 'X' || b == '^' || b == '_':
				// OSC, DCS, SOS, PM, APC: string sequences until BEL/ST.
				s.inString = true
			case b >= 0x20 && b <= 0x2f:
				// Intermediate byte (e.g. ESC ( B); final byte follows.
				s.inEscape = true
			default:
				// Two-byte escape (ESC + final byte); already consumed.
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
	// keepANSI retains escape sequences instead of stripping them. Renderers
	// that strip ANSI themselves and self-cap against the engine compression
	// contract need the unstripped stream: the contract budgets against the
	// raw byte cost, and a pre-stripped buffer makes ANSI-heavy output look
	// several times smaller than what the contract will measure.
	keepANSI bool
}

func (b *textBuffer) Consume(chunk []byte) {
	if b.keepANSI {
		b.consumeRaw(chunk)
		return
	}
	b.stripper.Consume(chunk, func(c byte) {
		if c != '\r' {
			b.builder.WriteByte(c)
		}
	})
}

func (b *textBuffer) consumeRaw(chunk []byte) {
	for _, c := range chunk {
		if c != '\r' {
			b.builder.WriteByte(c)
		}
	}
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
