package engine

import (
	"strings"
)

func combineStreams(stdout, stderr string) string {
	return CombineStreams(stdout, stderr)
}

func CombineStreams(stdout, stderr string) string {
	stdout = strings.TrimSpace(stdout)
	stderr = strings.TrimSpace(stderr)
	switch {
	case stdout == "" && stderr == "":
		return ""
	case stdout == "":
		return stderr
	case stderr == "":
		return stdout
	default:
		return stdout + "\n" + stderr
	}
}

func sanitizeFileName(value string) string {
	return SanitizeFileName(value)
}

func SanitizeFileName(value string) string {
	value = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, value)
	value = strings.Trim(value, "_")
	if value == "" {
		return "output"
	}
	if len(value) > 48 {
		return value[:48]
	}
	return value
}
