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
	for {
		jsonPath := filepath.Join(dir, jsonFileName)
		if _, err := stat(jsonPath); err == nil {
			return jsonPath, FormatJSON, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", FormatUnknown, err
		}

		for _, name := range []string{yamlFileName, ymlFileName} {
			yamlPath := filepath.Join(dir, name)
			if _, err := stat(yamlPath); err == nil {
				return yamlPath, FormatYAML, nil
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
