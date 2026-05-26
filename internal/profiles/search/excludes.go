package search

import "github.com/devr-tools/szr/internal/filters"

func defaultSearchExcludeDirs() []string {
	dirs := filters.DefaultSearchNoiseDirs()
	dirs = append(dirs,
		".gradle",
		".mypy_cache",
		".nox",
		".nuxt",
		".output",
		".parcel-cache",
		".pnpm-store",
		".ruff_cache",
		".svelte-kit",
		".venv",
		".yarn",
		"out",
		"tmp",
	)
	return dirs
}

func defaultSearchExcludeGlobs() []string {
	dirs := defaultSearchExcludeDirs()
	globs := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		globs = append(globs, "!"+dir+"/**")
	}
	return globs
}
