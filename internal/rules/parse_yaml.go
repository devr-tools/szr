package rules

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseYAML(data []byte) (File, error) {
	var (
		file              File
		currentProfile    *Profile
		currentPreference *Preference
		topSection        string
		section           string
		listField         string
		lineNo            int
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
			topSection = ""
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
				topSection = "profiles"
				if hasValue && parseYAMLScalar(value) != "" {
					return File{}, fmt.Errorf("line %d: profiles must be declared as a block list", lineNo)
				}
			case "preferences":
				topSection = "preferences"
				if hasValue && parseYAMLScalar(value) != "" {
					return File{}, fmt.Errorf("line %d: preferences must be declared as a block list", lineNo)
				}
			default:
				return File{}, fmt.Errorf("line %d: unsupported top-level key %q", lineNo, key)
			}
		case 2:
			if !strings.HasPrefix(trimmed, "- ") {
				return File{}, fmt.Errorf("line %d: expected %s list item", lineNo, topSectionName(topSection))
			}
			currentProfile = nil
			currentPreference = nil
			switch topSection {
			case "profiles":
				file.Profiles = append(file.Profiles, Profile{})
				currentProfile = &file.Profiles[len(file.Profiles)-1]
			case "preferences":
				file.Preferences = append(file.Preferences, Preference{})
				currentPreference = &file.Preferences[len(file.Preferences)-1]
			default:
				return File{}, fmt.Errorf("line %d: list item without profiles or preferences section", lineNo)
			}
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
			if currentProfile != nil {
				if err := setProfileField(currentProfile, key, value, hasValue, &section, &listField, lineNo); err != nil {
					return File{}, err
				}
			} else {
				if err := setPreferenceField(currentPreference, key, value, hasValue, &section, &listField, lineNo); err != nil {
					return File{}, err
				}
			}
		case 4:
			if currentProfile == nil && currentPreference == nil {
				return File{}, fmt.Errorf("line %d: field without profile or preference item", lineNo)
			}
			key, value, hasValue, err := parseYAMLKeyValue(trimmed)
			if err != nil {
				return File{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			if currentProfile != nil {
				if err := setProfileField(currentProfile, key, value, hasValue, &section, &listField, lineNo); err != nil {
					return File{}, err
				}
			} else {
				if err := setPreferenceField(currentPreference, key, value, hasValue, &section, &listField, lineNo); err != nil {
					return File{}, err
				}
			}
		case 6:
			if currentProfile == nil && currentPreference == nil {
				return File{}, fmt.Errorf("line %d: nested field without profile or preference item", lineNo)
			}
			if section == "explain" {
				if !strings.HasPrefix(trimmed, "- ") {
					return File{}, fmt.Errorf("line %d: explain entries must be list items", lineNo)
				}
				if currentProfile != nil {
					currentProfile.Explain = append(currentProfile.Explain, parseYAMLScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))))
				} else {
					currentPreference.Explain = append(currentPreference.Explain, parseYAMLScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))))
				}
				continue
			}
			if section == "match" || section == "rewrite" || section == "render" {
				key, value, hasValue, err := parseYAMLKeyValue(trimmed)
				if err != nil {
					return File{}, fmt.Errorf("line %d: %w", lineNo, err)
				}
				if currentProfile != nil {
					if err := setSectionField(currentProfile, section, key, value, hasValue, &listField, lineNo); err != nil {
						return File{}, err
					}
				} else {
					if err := setPreferenceSectionField(currentPreference, section, key, value, hasValue, &listField, lineNo); err != nil {
						return File{}, err
					}
				}
				continue
			}
			return File{}, fmt.Errorf("line %d: unexpected nested block", lineNo)
		case 8:
			if (currentProfile == nil && currentPreference == nil) || section == "" || listField == "" {
				return File{}, fmt.Errorf("line %d: list item without active list field", lineNo)
			}
			if !strings.HasPrefix(trimmed, "- ") {
				return File{}, fmt.Errorf("line %d: expected list item", lineNo)
			}
			value := parseYAMLScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
			if currentProfile != nil {
				if err := appendSectionListValue(currentProfile, section, listField, value, lineNo); err != nil {
					return File{}, err
				}
			} else {
				if err := appendPreferenceSectionListValue(currentPreference, section, listField, value, lineNo); err != nil {
					return File{}, err
				}
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
