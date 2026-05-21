package bench

import (
	"fmt"
	"strings"

	"szr/internal/engine"
	"szr/internal/profiles"
)

type Harness struct {
	profiles map[string]engine.Profile
}

func NewHarness(maxLines int) *Harness {
	return NewHarnessWithProfiles(profiles.Builtins(maxLines))
}

func NewHarnessWithProfiles(list []engine.Profile) *Harness {
	index := make(map[string]engine.Profile, len(list))
	for _, profile := range list {
		index[profile.Name] = profile
	}
	return &Harness{profiles: index}
}

func (h *Harness) Profile(name string) (engine.Profile, bool) {
	profile, ok := h.profiles[name]
	return profile, ok
}

func (h *Harness) Render(fixture Fixture) (string, error) {
	profile, ok := h.Profile(fixture.ProfileName)
	if !ok {
		return "", fmt.Errorf("unknown profile %q", fixture.ProfileName)
	}

	rendered := fixture.RawCombined()
	if profile.Render != nil {
		rendered = profile.Render(fixture.Invocation, fixture.Execution)
	}
	if strings.TrimSpace(rendered) == "" {
		rendered = fixture.RawCombined()
	}
	return rendered, nil
}
