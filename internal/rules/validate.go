package rules

import (
	"fmt"
	"strings"
)

func Validate(file File) error {
	if file.Version != 0 && file.Version != 1 {
		return fmt.Errorf("unsupported project rule version %d", file.Version)
	}
	if len(file.Profiles) == 0 && len(file.Preferences) == 0 {
		return fmt.Errorf("project rules must define at least one profile or preference")
	}

	names := make(map[string]struct{}, len(file.Profiles)+len(file.Preferences))
	for i, profile := range file.Profiles {
		if err := validateName("profile", i, profile.Name, names); err != nil {
			return err
		}
		if err := validateMatch("profile", profile.Name, profile.Match); err != nil {
			return err
		}
		if err := validateRewrite(profile); err != nil {
			return err
		}
		if err := validateRender(profile); err != nil {
			return err
		}
	}
	for i, preference := range file.Preferences {
		if err := validateName("preference", i, preference.Name, names); err != nil {
			return err
		}
		if err := validateMatch("preference", preference.Name, preference.Match); err != nil {
			return err
		}
		if err := validatePreference(preference); err != nil {
			return err
		}
	}
	return nil
}

func validateName(kind string, index int, name string, seen map[string]struct{}) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%s %d: name is required", kind, index)
	}
	if _, exists := seen[name]; exists {
		return fmt.Errorf("%s %d: duplicate %s name %q", kind, index, kind, name)
	}
	seen[name] = struct{}{}
	return nil
}

func validateMatch(kind, name string, match Match) error {
	if len(match.CommandPrefix) == 0 && len(match.DisplayPrefix) == 0 {
		return fmt.Errorf("%s %q: match.command_prefix or match.display_prefix is required", kind, name)
	}
	return nil
}

func validateRewrite(profile Profile) error {
	return validateRewriteSpec("profile", profile.Name, profile.Rewrite, false)
}

func validatePreference(preference Preference) error {
	return validateRewriteSpec("preference", preference.Name, preference.Rewrite, true)
}

func validateRewriteSpec(kind, name string, rewrite Rewrite, requireRewrite bool) error {
	mode := rewrite.Mode
	if mode == "" {
		mode = "append"
	}
	switch mode {
	case "append", "replace":
	default:
		return fmt.Errorf("%s %q: unsupported rewrite mode %q", kind, name, rewrite.Mode)
	}
	placement := rewrite.Placement
	if placement == "" {
		placement = "append"
	}
	switch placement {
	case "append", "before-terminator":
	default:
		return fmt.Errorf("%s %q: unsupported rewrite placement %q", kind, name, rewrite.Placement)
	}
	if requireRewrite && len(rewrite.Args) == 0 {
		return fmt.Errorf("%s %q: rewrite.args is required", kind, name)
	}
	if rewrite.Mode != "" && len(rewrite.Args) == 0 {
		return fmt.Errorf("%s %q: rewrite.args is required when rewrite.mode is set", kind, name)
	}
	return nil
}

func validateRender(profile Profile) error {
	mode := profile.Render.Mode
	if mode == "" {
		mode = "compact"
	}
	switch mode {
	case "compact", "failure", "passthrough":
	default:
		return fmt.Errorf("profile %q: unsupported render mode %q", profile.Name, profile.Render.Mode)
	}
	if profile.Render.MaxLines < 0 {
		return fmt.Errorf("profile %q: render.max_lines must be >= 0", profile.Name)
	}
	return nil
}
