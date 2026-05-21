package rules

import (
	"fmt"
	"strings"
)

func topSectionName(section string) string {
	if section == "" {
		return "rule"
	}
	return strings.TrimSuffix(section, "s")
}

func parseYAMLKeyValue(line string) (string, string, bool, error) {
	index := strings.Index(line, ":")
	if index < 0 {
		return "", "", false, fmt.Errorf("expected key: value")
	}
	key := strings.TrimSpace(line[:index])
	if key == "" {
		return "", "", false, fmt.Errorf("missing key")
	}
	value := strings.TrimSpace(line[index+1:])
	return key, value, value != "", nil
}

func parseYAMLStringList(value string) ([]string, error) {
	scalar := strings.TrimSpace(value)
	if scalar == "" {
		return nil, nil
	}
	if !strings.HasPrefix(scalar, "[") || !strings.HasSuffix(scalar, "]") {
		return []string{parseYAMLScalar(scalar)}, nil
	}
	body := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(scalar, "["), "]"))
	if body == "" {
		return nil, nil
	}
	parts := strings.Split(body, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		values = append(values, parseYAMLScalar(part))
	}
	return values, nil
}

func parseYAMLScalar(value string) string {
	scalar := strings.TrimSpace(value)
	if len(scalar) >= 2 {
		if scalar[0] == '"' && scalar[len(scalar)-1] == '"' {
			return scalar[1 : len(scalar)-1]
		}
		if scalar[0] == '\'' && scalar[len(scalar)-1] == '\'' {
			return scalar[1 : len(scalar)-1]
		}
	}
	return scalar
}
