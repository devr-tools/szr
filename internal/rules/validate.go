package rules

import (
	"fmt"
	"strings"
)

func Validate(file File) error {
	if file.Version != 0 && file.Version != 1 {
		return fmt.Errorf("unsupported project rule version %d", file.Version)
	}
	if len(file.Profiles) == 0 {
		return fmt.Errorf("project rules must define at least one profile")
	}

	names := make(map[string]struct{}, len(file.Profiles))
	for i, profile := range file.Profiles {
		if strings.TrimSpace(profile.Name) == "" {
			return fmt.Errorf("profile %d: name is required", i)
		}
		if _, exists := names[profile.Name]; exists {
			return fmt.Errorf("profile %d: duplicate profile name %q", i, profile.Name)
		}
		names[profile.Name] = struct{}{}

		if len(profile.Match.CommandPrefix) == 0 && len(profile.Match.DisplayPrefix) == 0 {
			return fmt.Errorf("profile %q: match.command_prefix or match.display_prefix is required", profile.Name)
		}
		if err := validateRewrite(profile); err != nil {
			return err
		}
		if err := validateRender(profile); err != nil {
			return err
		}
	}
	return nil
}

func validateRewrite(profile Profile) error {
	mode := profile.Rewrite.Mode
	if mode == "" {
		mode = "append"
	}
	switch mode {
	case "append", "replace":
	default:
		return fmt.Errorf("profile %q: unsupported rewrite mode %q", profile.Name, profile.Rewrite.Mode)
	}
	if profile.Rewrite.Mode != "" && len(profile.Rewrite.Args) == 0 {
		return fmt.Errorf("profile %q: rewrite.args is required when rewrite.mode is set", profile.Name)
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
