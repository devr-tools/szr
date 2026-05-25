package rules

import (
	"fmt"
	"strconv"
)

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

func setPreferenceField(preference *Preference, key, value string, hasValue bool, section *string, listField *string, lineNo int) error {
	*listField = ""
	switch key {
	case "name":
		if !hasValue {
			return fmt.Errorf("line %d: name requires a value", lineNo)
		}
		preference.Name = parseYAMLScalar(value)
	case "description":
		if !hasValue {
			return fmt.Errorf("line %d: description requires a value", lineNo)
		}
		preference.Description = parseYAMLScalar(value)
	case "explain":
		*section = "explain"
		if hasValue {
			values, err := parseYAMLStringList(value)
			if err != nil {
				return fmt.Errorf("line %d: %w", lineNo, err)
			}
			preference.Explain = append(preference.Explain[:0], values...)
			*section = ""
		}
	case "match", "rewrite":
		if hasValue {
			return fmt.Errorf("line %d: %s must be declared as a nested block", lineNo, key)
		}
		*section = key
	default:
		return fmt.Errorf("line %d: unsupported preference key %q", lineNo, key)
	}
	return nil
}

func setSectionField(profile *Profile, section, key, value string, hasValue bool, listField *string, lineNo int) error {
	*listField = ""
	switch section {
	case "match":
		return setMatchSectionField(profile, key, value, hasValue, listField, lineNo)
	case "rewrite":
		return setRewriteSectionField(profile, key, value, hasValue, listField, lineNo)
	case "render":
		return setRenderSectionField(profile, key, value, hasValue, lineNo)
	default:
		return fmt.Errorf("line %d: unsupported nested section %q", lineNo, section)
	}
}

func setMatchSectionField(profile *Profile, key, value string, hasValue bool, listField *string, lineNo int) error {
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
		return nil
	default:
		return fmt.Errorf("line %d: unsupported match key %q", lineNo, key)
	}
}

func setRewriteSectionField(profile *Profile, key, value string, hasValue bool, listField *string, lineNo int) error {
	switch key {
	case "mode":
		if !hasValue {
			return fmt.Errorf("line %d: rewrite.mode requires a value", lineNo)
		}
		profile.Rewrite.Mode = parseYAMLScalar(value)
	case "placement":
		if !hasValue {
			return fmt.Errorf("line %d: rewrite.placement requires a value", lineNo)
		}
		profile.Rewrite.Placement = parseYAMLScalar(value)
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
	return nil
}

func setRenderSectionField(profile *Profile, key, value string, hasValue bool, lineNo int) error {
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
	return nil
}

func setPreferenceSectionField(preference *Preference, section, key, value string, hasValue bool, listField *string, lineNo int) error {
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
			assignMatchList(&preference.Match, key, values)
		default:
			return fmt.Errorf("line %d: unsupported match key %q", lineNo, key)
		}
	case "rewrite":
		switch key {
		case "mode":
			if !hasValue {
				return fmt.Errorf("line %d: rewrite.mode requires a value", lineNo)
			}
			preference.Rewrite.Mode = parseYAMLScalar(value)
		case "placement":
			if !hasValue {
				return fmt.Errorf("line %d: rewrite.placement requires a value", lineNo)
			}
			preference.Rewrite.Placement = parseYAMLScalar(value)
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
				preference.Rewrite.Args = values
			} else {
				preference.Rewrite.SkipIfHasAny = values
			}
		default:
			return fmt.Errorf("line %d: unsupported rewrite key %q", lineNo, key)
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

func appendPreferenceSectionListValue(preference *Preference, section, field, value string, lineNo int) error {
	switch section {
	case "match":
		assignMatchList(&preference.Match, field, append(readMatchList(preference.Match, field), value))
		return nil
	case "rewrite":
		switch field {
		case "args":
			preference.Rewrite.Args = append(preference.Rewrite.Args, value)
			return nil
		case "skip_if_has_any":
			preference.Rewrite.SkipIfHasAny = append(preference.Rewrite.SkipIfHasAny, value)
			return nil
		}
	case "explain":
		preference.Explain = append(preference.Explain, value)
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
