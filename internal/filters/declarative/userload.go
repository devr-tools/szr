package declarative

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UserMatch routes invocations to a user-defined spec. Keys mirror the
// project-rule match section; at least one positive rule is required.
type UserMatch struct {
	CommandPrefix []string `json:"command_prefix,omitempty"`
	DisplayPrefix []string `json:"display_prefix,omitempty"`
	AnyArgs       []string `json:"any_args,omitempty"`
	ExcludeArgs   []string `json:"exclude_args,omitempty"`
}

func (m UserMatch) Empty() bool {
	return len(m.CommandPrefix) == 0 && len(m.DisplayPrefix) == 0 && len(m.AnyArgs) == 0
}

// UserSpec is a builtin-format Spec plus the match section that routes
// commands to it and the file it was loaded from.
type UserSpec struct {
	Spec
	Match UserMatch `json:"match"`
	Path  string    `json:"-"`
}

// LoadUserSpecs reads *.json spec files from dir in filename order. Invalid
// files are skipped and reported as warnings; a missing dir yields nothing.
func LoadUserSpecs(dir string) ([]UserSpec, []string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, []string{fmt.Sprintf("read filters dir %s: %v", dir, err)}
	}
	return loadUserSpecEntries(dir, entries)
}

func loadUserSpecEntries(dir string, entries []os.DirEntry) ([]UserSpec, []string) {
	specs := make([]UserSpec, 0, len(entries))
	var warnings []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		spec, err := loadUserSpecFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			warnings = append(warnings, err.Error())
			continue
		}
		specs = append(specs, spec)
	}
	return specs, warnings
}

// HasUserSpecFiles reports whether dir contains any .json spec files.
func HasUserSpecFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			return true
		}
	}
	return false
}

func loadUserSpecFile(path string) (UserSpec, error) {
	spec, err := parseUserSpecFile(path)
	if err != nil {
		return UserSpec{}, err
	}
	if err := validateUserSpec(spec, path); err != nil {
		return UserSpec{}, err
	}
	spec.Path = path
	return spec, nil
}

func parseUserSpecFile(path string) (UserSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return UserSpec{}, fmt.Errorf("read filter spec %s: %w", path, err)
	}
	var spec UserSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return UserSpec{}, fmt.Errorf("parse filter spec %s: %w", path, err)
	}
	if spec.Name == "" {
		spec.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	return spec, nil
}

func validateUserSpec(spec UserSpec, path string) error {
	if err := Validate(spec.Spec); err != nil {
		return fmt.Errorf("validate filter spec %s: %w", path, err)
	}
	if spec.Match.Empty() {
		return fmt.Errorf("filter spec %s: match needs command_prefix, display_prefix, or any_args", path)
	}
	if _, err := compileValidatedSpec(spec.Spec, Options{}); err != nil {
		return fmt.Errorf("compile filter spec %s: %w", path, err)
	}
	return nil
}
