package rules

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

func ParseFile(path string, data []byte) (File, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return ParseJSON(data)
	case ".yaml", ".yml":
		return ParseYAML(data)
	default:
		return File{}, fmt.Errorf("unsupported project rule file format: %s", path)
	}
}

func ParseJSON(data []byte) (File, error) {
	var file File
	if err := json.Unmarshal(data, &file); err != nil {
		return File{}, err
	}
	if err := Validate(file); err != nil {
		return File{}, err
	}
	return file, nil
}
