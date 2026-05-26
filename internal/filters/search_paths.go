package filters

import "strings"

var defaultSearchNoiseDirs = []string{
	".git",
	".next",
	".turbo",
	".cache",
	"__pycache__",
	"build",
	"coverage",
	"dist",
	"node_modules",
	"target",
	"vendor",
}

func DefaultSearchNoiseDirs() []string {
	return append([]string{}, defaultSearchNoiseDirs...)
}

func DefaultRipgrepExcludeGlobs() []string {
	globs := make([]string, 0, len(defaultSearchNoiseDirs))
	for _, dir := range defaultSearchNoiseDirs {
		globs = append(globs, "!"+dir+"/**")
	}
	return globs
}

func SearchNoiseBucket(path string) string {
	for _, part := range strings.Split(normalizeSearchPath(path), "/") {
		for _, dir := range defaultSearchNoiseDirs {
			if part == dir {
				return dir
			}
		}
	}
	return ""
}

func IsSearchNoisePath(path string) bool {
	return SearchNoiseBucket(path) != ""
}

func normalizeSearchPath(path string) string {
	path = strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	absolute := strings.HasPrefix(path, "/")
	path = strings.TrimPrefix(path, "./")
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}
	path = strings.TrimRight(path, "/")
	if absolute {
		path = "/" + strings.TrimLeft(path, "/")
	}
	return path
}
