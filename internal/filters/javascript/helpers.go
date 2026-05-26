package javascript

import shared "github.com/devr-tools/szr/internal/filters"

func StripANSI(input string) string {
	return shared.StripANSI(input)
}

func CompactLines(input string, maxLines int) string {
	return shared.CompactLines(input, maxLines)
}

func clip(input string, max int) string {
	return shared.Clip(input, max)
}

func uniqueStrings(values []string) []string {
	return shared.UniqueStrings(values)
}

func nonEmptyLines(input string) []string {
	return shared.NonEmptyLines(input)
}

func NewBufferedTextReducer(stdoutEnabled, stderrEnabled bool, render func(string) string) *shared.BufferedTextReducer {
	return shared.NewBufferedTextReducer(stdoutEnabled, stderrEnabled, render)
}
