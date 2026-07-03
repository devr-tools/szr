package engine

import "strings"

// shellSegment is one top-level `&&`/`;` segment of a shell command string:
// its split words, the byte offset of its first character in the original
// string, and whether every word re-quotes losslessly (no expansions, globs,
// or tildes were involved).
type shellSegment struct {
	words []string
	start int
	safe  bool
}

type shellSegmentParser struct {
	input     string
	segments  []shellSegment
	words     []string
	word      strings.Builder
	haveWord  bool
	wordRisky bool
	segStart  int
	segSafe   bool
	inSingle  bool
	inDouble  bool
}

// parseShellSegments splits a shell command string into top-level segments
// separated by `&&` or `;`, word-splitting each segment with quote and
// backslash awareness. It fails (ok=false) on anything beyond that shape:
// pipes, redirection, command substitution, subshells, background jobs,
// `||`, comments, newlines, and unterminated quotes. Words that used
// expansion-capable syntax ($VAR, globs, leading tildes, double-quoted
// expansions) mark their segment as not literal-safe.
func parseShellSegments(input string) ([]shellSegment, bool) {
	parser := &shellSegmentParser{input: input, segStart: -1, segSafe: true}
	i := 0
	for i < len(input) {
		next, ok := parser.step(i)
		if !ok {
			return nil, false
		}
		i = next
	}
	if parser.inSingle || parser.inDouble {
		return nil, false
	}
	parser.endSegment()
	return parser.segments, true
}

func (p *shellSegmentParser) step(i int) (int, bool) {
	switch {
	case p.inSingle:
		return p.stepSingleQuote(i)
	case p.inDouble:
		return p.stepDoubleQuote(i)
	default:
		return p.stepBare(i)
	}
}

func (p *shellSegmentParser) stepSingleQuote(i int) (int, bool) {
	if p.input[i] == '\'' {
		p.inSingle = false
		return i + 1, true
	}
	p.word.WriteByte(p.input[i])
	return i + 1, true
}

func (p *shellSegmentParser) stepDoubleQuote(i int) (int, bool) {
	c := p.input[i]
	switch c {
	case '"':
		p.inDouble = false
		return i + 1, true
	case '`':
		return 0, false
	case '$', '!':
		if c == '$' && p.peek(i+1) == '(' {
			return 0, false
		}
		p.wordRisky = true
		p.word.WriteByte(c)
		return i + 1, true
	case '\\':
		return p.consumeDoubleQuoteEscape(i)
	default:
		p.word.WriteByte(c)
		return i + 1, true
	}
}

// consumeDoubleQuoteEscape handles backslash inside double quotes, where it
// only escapes $, ", \, and backtick; before anything else the backslash is
// a literal character.
func (p *shellSegmentParser) consumeDoubleQuoteEscape(i int) (int, bool) {
	switch next := p.peek(i + 1); next {
	case '$', '"', '\\', '`':
		p.word.WriteByte(byte(next))
		return i + 2, true
	case -1, '\n':
		return 0, false
	default:
		p.word.WriteByte('\\')
		return i + 1, true
	}
}

// stepBare dispatches an unquoted byte: structural characters (separators,
// quotes, escapes, rejected operators) first, ordinary word bytes second.
func (p *shellSegmentParser) stepBare(i int) (int, bool) {
	if next, ok, handled := p.stepBareStructure(i); handled {
		return next, ok
	}
	return p.stepBareWordByte(i)
}

func (p *shellSegmentParser) stepBareStructure(i int) (int, bool, bool) {
	switch p.input[i] {
	case ' ', '\t':
		p.flushWord()
		return i + 1, true, true
	case ';', '&':
		next, ok := p.stepSeparator(i)
		return next, ok, true
	case '\'', '"':
		p.markWord(i)
		p.enterQuote(p.input[i])
		return i + 1, true, true
	case '\\':
		next, ok := p.consumeBareEscape(i)
		return next, ok, true
	case '|', '<', '>', '(', ')', '`', '\n', '\r':
		return 0, false, true
	default:
		return 0, false, false
	}
}

func (p *shellSegmentParser) stepSeparator(i int) (int, bool) {
	if p.input[i] == ';' {
		p.endSegment()
		return i + 1, true
	}
	if p.peek(i+1) != '&' {
		return 0, false
	}
	p.endSegment()
	return i + 2, true
}

func (p *shellSegmentParser) enterQuote(c byte) {
	if c == '\'' {
		p.inSingle = true
		return
	}
	p.inDouble = true
}

func (p *shellSegmentParser) stepBareWordByte(i int) (int, bool) {
	c := p.input[i]
	if c == '$' && p.peek(i+1) == '(' {
		return 0, false
	}
	if c == '#' && !p.haveWord {
		// A comment would swallow the rest of the string.
		return 0, false
	}
	if p.isRiskyBareByte(c) {
		return p.writeRisky(i, c), true
	}
	p.markWord(i)
	p.word.WriteByte(c)
	return i + 1, true
}

// isRiskyBareByte reports expansion-capable bytes that keep a word from
// re-quoting losslessly; a tilde only expands at the start of a word.
func (p *shellSegmentParser) isRiskyBareByte(c byte) bool {
	if c == '~' {
		return !p.haveWord
	}
	return strings.IndexByte("$#*?[]{}!^", c) >= 0
}

func (p *shellSegmentParser) writeRisky(i int, c byte) int {
	p.markWord(i)
	p.wordRisky = true
	p.word.WriteByte(c)
	return i + 1
}

func (p *shellSegmentParser) consumeBareEscape(i int) (int, bool) {
	next := p.peek(i + 1)
	if next == -1 || next == '\n' {
		return 0, false
	}
	p.markWord(i)
	p.word.WriteByte(byte(next))
	return i + 2, true
}

func (p *shellSegmentParser) peek(i int) int {
	if i >= len(p.input) {
		return -1
	}
	return int(p.input[i])
}

func (p *shellSegmentParser) markWord(i int) {
	if p.segStart < 0 {
		p.segStart = i
	}
	p.haveWord = true
}

func (p *shellSegmentParser) flushWord() {
	if !p.haveWord {
		return
	}
	p.words = append(p.words, p.word.String())
	if p.wordRisky {
		p.segSafe = false
	}
	p.word.Reset()
	p.haveWord = false
	p.wordRisky = false
}

func (p *shellSegmentParser) endSegment() {
	p.flushWord()
	if len(p.words) > 0 {
		p.segments = append(p.segments, shellSegment{words: p.words, start: p.segStart, safe: p.segSafe})
	}
	p.words = nil
	p.segStart = -1
	p.segSafe = true
}
