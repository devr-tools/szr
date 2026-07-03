package engine

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/config"
)

func TestRenderExecutionAppliesCompressionContract(t *testing.T) {
	t.Parallel()

	raw := strings.Repeat("token ", 400)
	profile := Profile{
		Name:   "contract-render",
		Budget: OutputBudget{MaxLines: 12, MaxTokens: 32},
		Render: func(_ Invocation, exec Execution) string {
			return exec.Stdout
		},
	}
	rendered := RenderExecution(profile, Invocation{Advanced: configAdvancedForTests()}, Execution{Stdout: raw}, 12, false)
	allowed := compressionContractAllowedTokens(rendered.RawTokens, profile.Budget)
	if rendered.FilteredTokens > allowed {
		t.Fatalf("expected rendered tokens <= %d, got %d (%q)", allowed, rendered.FilteredTokens, rendered.Text)
	}
}

func configAdvancedForTests() config.Advanced {
	return config.Advanced{CompressionContract: true, CompactArtifactRefs: true}
}
