package filters

import (
	"strconv"
	"strings"
)

var relativeAgeUnits = []struct {
	keyword string
	suffix  string
}{
	{"second", "s"},
	{"minute", "m"},
	{"hour", "h"},
	{"day", "d"},
	{"week", "w"},
	{"month", "mo"},
	{"year", "y"},
}

// CompactRelativeAge folds human relative ages such as "2 weeks ago" or
// "About an hour ago" into short forms like "2w" and "1h". Unrecognized
// inputs pass through trimmed but otherwise untouched.
func CompactRelativeAge(input string) string {
	trimmed := strings.TrimSpace(input)
	lower := strings.TrimSuffix(strings.ToLower(trimmed), " ago")
	count, unitText, ok := splitRelativeAge(lower)
	if !ok {
		return trimmed
	}
	for _, unit := range relativeAgeUnits {
		if strings.Contains(unitText, unit.keyword) {
			return strconv.Itoa(count) + unit.suffix
		}
	}
	return trimmed
}

func splitRelativeAge(lower string) (int, string, bool) {
	lower = strings.TrimPrefix(lower, "about ")
	for _, prefix := range []string{"an ", "a "} {
		if strings.HasPrefix(lower, prefix) {
			return 1, strings.TrimPrefix(lower, prefix), true
		}
	}
	first, rest, ok := strings.Cut(lower, " ")
	if !ok {
		return 0, "", false
	}
	count, err := strconv.Atoi(first)
	if err != nil {
		return 0, "", false
	}
	return count, rest, true
}
