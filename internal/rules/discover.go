package rules

import (
	"errors"
	"os"
	"path/filepath"
)

const (
	jsonFileName = ".szr.json"
	yamlFileName = ".szr.yaml"
	ymlFileName  = ".szr.yml"
)

func Discover(startDir string) (string, Format, error) {
	return DiscoverWith(startDir, os.Stat)
}

func DiscoverWith(startDir string, stat func(string) (os.FileInfo, error)) (string, Format, error) {
	dir := filepath.Clean(startDir)
	candidates := []struct {
		name   string
		format Format
	}{
		{jsonFileName, FormatJSON},
		{yamlFileName, FormatYAML},
		{ymlFileName, FormatYAML},
	}
	for {
		for _, candidate := range candidates {
			path := filepath.Join(dir, candidate.name)
			if _, err := stat(path); err == nil {
				return path, candidate.format, nil
			} else if !errors.Is(err, os.ErrNotExist) {
				return "", FormatUnknown, err
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", FormatUnknown, nil
		}
		dir = parent
	}
}
