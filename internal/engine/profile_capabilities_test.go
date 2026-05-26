package engine

import "testing"

func TestBuildRunOptionsUsesRequireFullCaptureCapability(t *testing.T) {
	profile := Profile{
		Render:       func(Invocation, Execution) string { return "" },
		Capabilities: ProfileCapabilities{RequireFullCapture: true},
	}

	options := buildRunOptions(Invocation{}, profile, false, nil)
	if !options.captureStdout || !options.captureStderr {
		t.Fatalf("expected full capture when capability requires it: %+v", options)
	}
}

func TestBuildRunOptionsUsesReducerRecoveryCaptureRequirement(t *testing.T) {
	options := buildRunOptions(Invocation{}, Profile{}, false, &recoveryStubReducer{
		kind:              RecoveryKindFullOutput,
		requireRawCapture: true,
	})
	if !options.captureStdout || !options.captureStderr {
		t.Fatalf("expected full capture when reducer may need recovery artifact: %+v", options)
	}
}

func TestAnnotateProfilesCapabilitiesProvidesDefaults(t *testing.T) {
	profiles := annotateProfilesCapabilities([]Profile{{
		Name:         "medium-profile",
		Confidence:   ConfidenceMedium,
		StreamRender: func(Invocation, OutputBudget) StreamReducer { return nil },
		Prepare:      func(inv Invocation) []string { return inv.Command },
	}})

	if len(profiles) != 1 {
		t.Fatalf("unexpected profile count: %d", len(profiles))
	}
	caps := profiles[0].Capabilities
	if caps.FastPathBypass != FastPathBypassSmallOutput {
		t.Fatalf("unexpected fast-path default: %q", caps.FastPathBypass)
	}
	if !caps.AllowFailureEscape {
		t.Fatal("expected medium-confidence profile to allow failure escape by default")
	}
	if !caps.RequireFullCapture {
		t.Fatal("expected medium-confidence profile to require full capture by default")
	}
	if !caps.InjectsPrepareArgs {
		t.Fatal("expected prepare-enabled profile to mark prepare arg injection by default")
	}
}
