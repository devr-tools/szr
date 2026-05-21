package rules

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseYAML(data []byte) (File, error) {
	var (
		file           File
		currentProfile *Profile
		section        string
		listField      string
		lineNo         int
	)

	for _, rawLine := range strings.Split(string(data), "\n") {
		lineNo++
		line := strings.TrimRight(rawLine, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.ContainsRune(line, '\t') {
			return File{}, fmt.Errorf("line %d: tabs are not supported in yaml rules", lineNo)
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))
		switch indent {
		case 0:
			section = ""
			listField = ""
			key, value, hasValue, err := parseYAMLKeyValue(trimmed)
			if err != nil {
				return File{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			switch key {
			case "version":
				if !hasValue {
					return File{}, fmt.Errorf("line %d: version requires a value", lineNo)
				}
				version, err := strconv.Atoi(parseYAMLScalar(value))
				if err != nil {
					return File{}, fmt.Errorf("line %d: invalid version %q", lineNo, value)
				}
				file.Version = version
			case "profiles":
				if hasValue && parseYAMLScalar(value) != "" {
					return File{}, fmt.Errorf("line %d: profiles must be declared as a block list", lineNo)
				}
			default:
				return File{}, fmt.Errorf("line %d: unsupported top-level key %q", lineNo, key)
			}
		case 2:
			if !strings.HasPrefix(trimmed, "- ") {
				return File{}, fmt.Errorf("line %d: expected profile list item", lineNo)
			}
			file.Profiles = append(file.Profiles, Profile{})
			currentProfile = &file.Profiles[len(file.Profiles)-1]
			section = ""
			listField = ""

			remainder := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			if remainder == "" {
				continue
			}
			key, value, hasValue, err := parseYAMLKeyValue(remainder)
			if err != nil {
				return File{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			if err := setProfileField(currentProfile, key, value, hasValue, &section, &listField, lineNo); err != nil {
				return File{}, err
			}
		case 4:
			if currentProfile == nil {
				return File{}, fmt.Errorf("line %d: profile field without profile item", lineNo)
			}
			key, value, hasValue, err := parseYAMLKeyValue(trimmed)
			if err != nil {
				return File{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			if err := setProfileField(currentProfile, key, value, hasValue, &section, &listField, lineNo); err != nil {
				return File{}, err
			}
		case 6:
			if currentProfile == nil {
				return File{}, fmt.Errorf("line %d: nested field without profile item", lineNo)
			}
			if section == "explain" {
				if !strings.HasPrefix(trimmed, "- ") {
					return File{}, fmt.Errorf("line %d: explain entries must be list items", lineNo)
				}
				currentProfile.Explain = append(currentProfile.Explain, parseYAMLScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))))
				continue
			}
			if section == "match" || section == "rewrite" || section == "render" {
				key, value, hasValue, err := parseYAMLKeyValue(trimmed)
				if err != nil {
					return File{}, fmt.Errorf("line %d: %w", lineNo, err)
				}
				if err := setSectionField(currentProfile, section, key, value, hasValue, &listField, lineNo); err != nil {
					return File{}, err
				}
				continue
			}
			return File{}, fmt.Errorf("line %d: unexpected nested block", lineNo)
		case 8:
			if currentProfile == nil || section == "" || listField == "" {
				return File{}, fmt.Errorf("line %d: list item without active list field", lineNo)
			}
			if !strings.HasPrefix(trimmed, "- ") {
				return File{}, fmt.Errorf("line %d: expected list item", lineNo)
			}
			value := parseYAMLScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
			if err := appendSectionListValue(currentProfile, section, listField, value, lineNo); err != nil {
				return File{}, err
			}
		default:
			return File{}, fmt.Errorf("line %d: unsupported indentation level %d", lineNo, indent)
		}
	}

	if err := Validate(file); err != nil {
		return File{}, err
	}
	return file, nil
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

func setProfileField(profile *Profile, key, value string, hasValue bool, section *string, listField *string, lineNo int) error {
	*listField = ""
	switch key {
	case "name":
		if !hasValue {
			return fmt.Errorf("line %d: name requires a value", lineNo)
		}
		profile.Name = parseYAMLScalar(value)
	case "description":
		if !hasValue {
			return fmt.Errorf("line %d: description requires a value", lineNo)
		}
		profile.Description = parseYAMLScalar(value)
	case "explain":
		*section = "explain"
		if hasValue {
			values, err := parseYAMLStringList(value)
			if err != nil {
				return fmt.Errorf("line %d: %w", lineNo, err)
			}
			profile.Explain = append(profile.Explain[:0], values...)
			*section = ""
		}
	case "match", "rewrite", "render":
		if hasValue {
			return fmt.Errorf("line %d: %s must be declared as a nested block", lineNo, key)
		}
		*section = key
	default:
		return fmt.Errorf("line %d: unsupported profile key %q", lineNo, key)
	}
	return nil
}

func setSectionField(profile *Profile, section, key, value string, hasValue bool, listField *string, lineNo int) error {
	*listField = ""
	switch section {
	case "match":
		switch key {
		case "command_prefix", "display_prefix", "all_args", "any_args", "exclude_args", "cwd_contains":
			if !hasValue {
				*listField = key
				return nil
			}
			values, err := parseYAMLStringList(value)
			if err != nil {
				return fmt.Errorf("line %d: %w", lineNo, err)
			}
			assignMatchList(&profile.Match, key, values)
		default:
			return fmt.Errorf("line %d: unsupported match key %q", lineNo, key)
		}
	case "rewrite":
		switch key {
		case "mode":
			if !hasValue {
				return fmt.Errorf("line %d: rewrite.mode requires a value", lineNo)
			}
			profile.Rewrite.Mode = parseYAMLScalar(value)
		case "args", "skip_if_has_any":
			if !hasValue {
				*listField = key
				return nil
			}
			values, err := parseYAMLStringList(value)
			if err != nil {
				return fmt.Errorf("line %d: %w", lineNo, err)
			}
			if key == "args" {
				profile.Rewrite.Args = values
			} else {
				profile.Rewrite.SkipIfHasAny = values
			}
		default:
			return fmt.Errorf("line %d: unsupported rewrite key %q", lineNo, key)
		}
	case "render":
		switch key {
		case "mode":
			if !hasValue {
				return fmt.Errorf("line %d: render.mode requires a value", lineNo)
			}
			profile.Render.Mode = parseYAMLScalar(value)
		case "max_lines":
			if !hasValue {
				return fmt.Errorf("line %d: render.max_lines requires a value", lineNo)
			}
			valueInt, err := strconv.Atoi(parseYAMLScalar(value))
			if err != nil {
				return fmt.Errorf("line %d: invalid render.max_lines %q", lineNo, value)
			}
			profile.Render.MaxLines = valueInt
		default:
			return fmt.Errorf("line %d: unsupported render key %q", lineNo, key)
		}
	default:
		return fmt.Errorf("line %d: unsupported nested section %q", lineNo, section)
	}
	return nil
}

func appendSectionListValue(profile *Profile, section, field, value string, lineNo int) error {
	switch section {
	case "match":
		assignMatchList(&profile.Match, field, append(readMatchList(profile.Match, field), value))
		return nil
	case "rewrite":
		switch field {
		case "args":
			profile.Rewrite.Args = append(profile.Rewrite.Args, value)
			return nil
		case "skip_if_has_any":
			profile.Rewrite.SkipIfHasAny = append(profile.Rewrite.SkipIfHasAny, value)
			return nil
		}
	case "explain":
		profile.Explain = append(profile.Explain, value)
		return nil
	}
	return fmt.Errorf("line %d: unsupported list field %s.%s", lineNo, section, field)
}

func assignMatchList(match *Match, field string, values []string) {
	switch field {
	case "command_prefix":
		match.CommandPrefix = values
	case "display_prefix":
		match.DisplayPrefix = values
	case "all_args":
		match.AllArgs = values
	case "any_args":
		match.AnyArgs = values
	case "exclude_args":
		match.ExcludeArgs = values
	case "cwd_contains":
		match.CwdContains = values
	}
}

func readMatchList(match Match, field string) []string {
	switch field {
	case "command_prefix":
		return match.CommandPrefix
	case "display_prefix":
		return match.DisplayPrefix
	case "all_args":
		return match.AllArgs
	case "any_args":
		return match.AnyArgs
	case "exclude_args":
		return match.ExcludeArgs
	case "cwd_contains":
		return match.CwdContains
	default:
		return nil
	}
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
