package rules

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseYAML(data []byte) (File, error) {
	parser := yamlRuleParser{}
	for _, rawLine := range strings.Split(string(data), "\n") {
		if err := parser.consumeLine(rawLine); err != nil {
			return File{}, err
		}
	}

	if err := Validate(parser.file); err != nil {
		return File{}, err
	}
	return parser.file, nil
}

type yamlRuleParser struct {
	file              File
	currentProfile    *Profile
	currentPreference *Preference
	topSection        string
	section           string
	listField         string
	lineNo            int
}

func (p *yamlRuleParser) consumeLine(rawLine string) error {
	p.lineNo++
	line := strings.TrimRight(rawLine, "\r")
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return nil
	}
	if strings.ContainsRune(line, '\t') {
		return fmt.Errorf("line %d: tabs are not supported in yaml rules", p.lineNo)
	}

	indent := len(line) - len(strings.TrimLeft(line, " "))
	switch indent {
	case 0:
		return p.consumeTopLevel(trimmed)
	case 2:
		return p.consumeListItem(trimmed)
	case 4:
		return p.consumeField(trimmed)
	case 6:
		return p.consumeNestedField(trimmed)
	case 8:
		return p.consumeListValue(trimmed)
	default:
		return fmt.Errorf("line %d: unsupported indentation level %d", p.lineNo, indent)
	}
}

func (p *yamlRuleParser) consumeTopLevel(trimmed string) error {
	p.topSection = ""
	p.section = ""
	p.listField = ""
	key, value, hasValue, err := parseYAMLKeyValue(trimmed)
	if err != nil {
		return fmt.Errorf("line %d: %w", p.lineNo, err)
	}
	switch key {
	case "version":
		return p.setVersion(value, hasValue)
	case "profiles", "preferences":
		return p.setTopSection(key, value, hasValue)
	default:
		return fmt.Errorf("line %d: unsupported top-level key %q", p.lineNo, key)
	}
}

func (p *yamlRuleParser) setVersion(value string, hasValue bool) error {
	if !hasValue {
		return fmt.Errorf("line %d: version requires a value", p.lineNo)
	}
	version, err := strconv.Atoi(parseYAMLScalar(value))
	if err != nil {
		return fmt.Errorf("line %d: invalid version %q", p.lineNo, value)
	}
	p.file.Version = version
	return nil
}

func (p *yamlRuleParser) setTopSection(key, value string, hasValue bool) error {
	p.topSection = key
	if hasValue && parseYAMLScalar(value) != "" {
		return fmt.Errorf("line %d: %s must be declared as a block list", p.lineNo, key)
	}
	return nil
}

func (p *yamlRuleParser) consumeListItem(trimmed string) error {
	if !strings.HasPrefix(trimmed, "- ") {
		return fmt.Errorf("line %d: expected %s list item", p.lineNo, topSectionName(p.topSection))
	}
	if err := p.beginListItem(); err != nil {
		return err
	}
	remainder := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
	if remainder == "" {
		return nil
	}
	return p.applyItemField(remainder)
}

func (p *yamlRuleParser) beginListItem() error {
	p.currentProfile = nil
	p.currentPreference = nil
	switch p.topSection {
	case "profiles":
		p.file.Profiles = append(p.file.Profiles, Profile{})
		p.currentProfile = &p.file.Profiles[len(p.file.Profiles)-1]
	case "preferences":
		p.file.Preferences = append(p.file.Preferences, Preference{})
		p.currentPreference = &p.file.Preferences[len(p.file.Preferences)-1]
	default:
		return fmt.Errorf("line %d: list item without profiles or preferences section", p.lineNo)
	}
	p.section = ""
	p.listField = ""
	return nil
}

func (p *yamlRuleParser) consumeField(trimmed string) error {
	if err := p.requireActiveItem("field without profile or preference item"); err != nil {
		return err
	}
	return p.applyItemField(trimmed)
}

func (p *yamlRuleParser) applyItemField(input string) error {
	key, value, hasValue, err := parseYAMLKeyValue(input)
	if err != nil {
		return fmt.Errorf("line %d: %w", p.lineNo, err)
	}
	if p.currentProfile != nil {
		return setProfileField(p.currentProfile, key, value, hasValue, &p.section, &p.listField, p.lineNo)
	}
	return setPreferenceField(p.currentPreference, key, value, hasValue, &p.section, &p.listField, p.lineNo)
}

func (p *yamlRuleParser) consumeNestedField(trimmed string) error {
	if err := p.requireActiveItem("nested field without profile or preference item"); err != nil {
		return err
	}
	if p.section == "explain" {
		return p.appendExplainValue(trimmed)
	}
	if p.section != "match" && p.section != "rewrite" && p.section != "render" {
		return fmt.Errorf("line %d: unexpected nested block", p.lineNo)
	}
	key, value, hasValue, err := parseYAMLKeyValue(trimmed)
	if err != nil {
		return fmt.Errorf("line %d: %w", p.lineNo, err)
	}
	if p.currentProfile != nil {
		return setSectionField(p.currentProfile, p.section, key, value, hasValue, &p.listField, p.lineNo)
	}
	return setPreferenceSectionField(p.currentPreference, p.section, key, value, hasValue, &p.listField, p.lineNo)
}

func (p *yamlRuleParser) appendExplainValue(trimmed string) error {
	if !strings.HasPrefix(trimmed, "- ") {
		return fmt.Errorf("line %d: explain entries must be list items", p.lineNo)
	}
	value := parseYAMLScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
	if p.currentProfile != nil {
		p.currentProfile.Explain = append(p.currentProfile.Explain, value)
	} else {
		p.currentPreference.Explain = append(p.currentPreference.Explain, value)
	}
	return nil
}

func (p *yamlRuleParser) consumeListValue(trimmed string) error {
	if (p.currentProfile == nil && p.currentPreference == nil) || p.section == "" || p.listField == "" {
		return fmt.Errorf("line %d: list item without active list field", p.lineNo)
	}
	if !strings.HasPrefix(trimmed, "- ") {
		return fmt.Errorf("line %d: expected list item", p.lineNo)
	}
	value := parseYAMLScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
	if p.currentProfile != nil {
		return appendSectionListValue(p.currentProfile, p.section, p.listField, value, p.lineNo)
	}
	return appendPreferenceSectionListValue(p.currentPreference, p.section, p.listField, value, p.lineNo)
}

func (p *yamlRuleParser) requireActiveItem(message string) error {
	if p.currentProfile == nil && p.currentPreference == nil {
		return fmt.Errorf("line %d: %s", p.lineNo, message)
	}
	return nil
}
