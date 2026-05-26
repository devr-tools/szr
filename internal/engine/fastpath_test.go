package engine

import (
	"testing"
	"time"
)

func TestDecideFastPathSkipsBypassOnNonZeroExit(t *testing.T) {
	profile := Profile{Name: "ripgrep"}

	decision := DecideFastPath(profile, Invocation{Command: []string{"rg", "todo", "."}}, 64, 16, 0, 1)
	if decision.BypassCompression {
		t.Fatalf("expected nonzero exit to disable bypass, got %+v", decision)
	}
	if decision.Reason != "" {
		t.Fatalf("expected no bypass reason on nonzero exit, got %q", decision.Reason)
	}
}

func TestDecideFastPathBypassesEmptyStderrOnlyProfile(t *testing.T) {
	profile := Profile{Name: "go-build", StreamPreference: StreamStderrOnly}

	decision := DecideFastPath(profile, Invocation{Command: []string{"go", "build", "./..."}}, 0, 0, 0, 0)
	if !decision.BypassCompression {
		t.Fatalf("expected empty stderr-only payload to bypass, got %+v", decision)
	}
	if decision.Reason != "stderr-only profile with empty stderr payload" {
		t.Fatalf("unexpected reason: %q", decision.Reason)
	}
}

func TestDecideFastPathFamilyAwareBypasses(t *testing.T) {
	tests := []struct {
		name       string
		profile    string
		command    []string
		rawBytes   int
		rawTokens  int
		wantBypass bool
		wantReason string
	}{
		{name: "tiny ripgrep", profile: "ripgrep", command: []string{"rg", "todo", "."}, rawBytes: 320, rawTokens: 80, wantBypass: true, wantReason: "tiny ripgrep output"},
		{name: "ripgrep over token limit", profile: "ripgrep", command: []string{"rg", "todo", "."}, rawBytes: 320, rawTokens: 81, wantBypass: false},
		{name: "tiny find", profile: "path-find", command: []string{"find", ".", "-name", "*.go"}, rawBytes: 300, rawTokens: 72, wantBypass: true, wantReason: "tiny find output"},
		{name: "short directory listing", profile: "directory-listing", command: []string{"ls", "."}, rawBytes: 288, rawTokens: 72, wantBypass: true, wantReason: "short directory listing"},
		{name: "directory listing over byte limit", profile: "directory-listing", command: []string{"ls", "."}, rawBytes: 289, rawTokens: 72, wantBypass: false},
		{name: "short tree gets tree-specific threshold", profile: "directory-listing", command: []string{"tree", "."}, rawBytes: 384, rawTokens: 96, wantBypass: true, wantReason: "short tree output"},
		{name: "tree over shape threshold", profile: "directory-listing", command: []string{"tree", "."}, rawBytes: 385, rawTokens: 96, wantBypass: false},
		{name: "tiny git diff", profile: "git-diff", command: []string{"git", "diff", "HEAD~1..HEAD"}, rawBytes: 256, rawTokens: 64, wantBypass: true, wantReason: "tiny git diff output"},
		{name: "git diff over byte limit", profile: "git-diff", command: []string{"git", "diff", "HEAD~1..HEAD"}, rawBytes: 257, rawTokens: 64, wantBypass: false},
		{name: "short git diff name list gets larger bypass", profile: "git-diff", command: []string{"git", "diff", "--name-only"}, rawBytes: 384, rawTokens: 96, wantBypass: true, wantReason: "short git diff name list"},
		{name: "git diff name list over shape threshold", profile: "git-diff", command: []string{"git", "diff", "--name-only"}, rawBytes: 385, rawTokens: 96, wantBypass: false},
		{name: "short tracked file list in generic summary", profile: "generic-summary", command: []string{"git", "ls-files", "*.go"}, rawBytes: 384, rawTokens: 96, wantBypass: true, wantReason: "short tracked file list"},
		{name: "git ls-files over token limit", profile: "generic-summary", command: []string{"git", "ls-files"}, rawBytes: 384, rawTokens: 97, wantBypass: false},
		{name: "plain git status uses base threshold", profile: "git-status", command: []string{"git", "status"}, rawBytes: 192, rawTokens: 48, wantBypass: true, wantReason: "short git status output"},
		{name: "short git status shape gets larger bypass", profile: "git-status", command: []string{"git", "status", "--short"}, rawBytes: 288, rawTokens: 72, wantBypass: true, wantReason: "short git status listing"},
		{name: "git status short over shape token limit", profile: "git-status", command: []string{"git", "status", "--short"}, rawBytes: 288, rawTokens: 73, wantBypass: false},
		{name: "plain git status keeps base threshold", profile: "git-status", command: []string{"git", "status"}, rawBytes: 193, rawTokens: 48, wantBypass: false},
		{name: "tiny git log", profile: "git-log", command: []string{"git", "log", "--oneline"}, rawBytes: 224, rawTokens: 56, wantBypass: true, wantReason: "tiny git log output"},
		{name: "short ripgrep files list", profile: "ripgrep-files", command: []string{"rg", "--files"}, rawBytes: 384, rawTokens: 96, wantBypass: true, wantReason: "short ripgrep file list"},
		{name: "ripgrep files list over shape threshold", profile: "ripgrep-files", command: []string{"rg", "--files"}, rawBytes: 385, rawTokens: 96, wantBypass: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := DecideFastPath(Profile{Name: tt.profile}, Invocation{Command: tt.command, Display: tt.command}, tt.rawBytes, tt.rawTokens, 0, 0)
			if decision.BypassCompression != tt.wantBypass {
				t.Fatalf("expected bypass=%t, got %+v", tt.wantBypass, decision)
			}
			if decision.Reason != tt.wantReason {
				t.Fatalf("expected reason %q, got %q", tt.wantReason, decision.Reason)
			}
		})
	}
}

func TestDecideFastPathFallsBackToGenericTinyOutput(t *testing.T) {
	decision := DecideFastPath(Profile{Name: "generic-summary"}, Invocation{Command: []string{"unknown"}}, defaultTinyOutputBypassBytes, defaultTinyOutputBypassTokens, 0, 0)
	if !decision.BypassCompression {
		t.Fatalf("expected generic tiny output bypass, got %+v", decision)
	}
	if decision.Reason != "tiny output fast path" {
		t.Fatalf("unexpected reason: %q", decision.Reason)
	}
}

func TestDecideFastPathSetsLatencyWarningIndependently(t *testing.T) {
	profile := Profile{Name: "directory-listing", LatencyBudget: 5 * time.Millisecond}

	decision := DecideFastPath(profile, Invocation{Command: []string{"ls", "."}}, 64, 16, 10*time.Millisecond, 0)
	if !decision.WarnLatency {
		t.Fatalf("expected latency warning, got %+v", decision)
	}
	if !decision.BypassCompression {
		t.Fatalf("expected family-aware bypass to remain active, got %+v", decision)
	}
}
