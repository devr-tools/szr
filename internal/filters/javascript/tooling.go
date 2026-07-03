package javascript

import (
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

// Selection tiers for workspace/tooling output: registry failures name the
// broken dependency; a file header together with its first finding is the
// minimal actionable unit per file (a header without a finding says nothing,
// a finding without its header names no file); remaining error findings
// outrank secondary details; run summaries close the render.
const (
	jsToolingTierCritical = iota
	jsToolingTierGroupLead
	jsToolingTierError
	jsToolingTierDetail
	jsToolingTierSummary
)

func SummarizeJSTooling(input string, maxLines int) string {
	return SummarizeJSToolingUnderContract(input, maxLines, false)
}

// SummarizeJSToolingUnderContract renders the tooling summary; when contract
// is true the render self-caps to the predicted engine compression-contract
// allowance (measured on the unstripped input) so file headers and error
// findings never lose a downstream density contest.
func SummarizeJSToolingUnderContract(input string, maxLines int, contract bool) string {
	clean := StripANSI(input)
	critical, grouped, summaries := collectJSToolingLines(clean)
	if len(critical)+len(grouped)+len(summaries) == 0 {
		return CompactLines(clean, maxLines)
	}

	allowance := 0
	if contract {
		allowance = shared.PredictedTokenAllowance(input, maxLines)
	}
	candidates := make([]shared.PriorityLine, 0, len(critical)+len(grouped)+len(summaries))
	for _, line := range critical {
		candidates = append(candidates, shared.PriorityLine{Text: line, Tier: jsToolingTierCritical})
	}
	candidates = append(candidates, grouped...)
	for _, line := range summaries {
		candidates = append(candidates, shared.PriorityLine{Text: line, Tier: jsToolingTierSummary})
	}
	return shared.FitPriorityLinesWithMarker(candidates, maxLines, allowance)
}

// collectJSToolingLines classifies tooling output into registry-critical
// lines, grouped findings (file headers attached to the findings under
// them), and run summaries. A bare file-path header is only kept once a
// finding follows it, so listers that print paths without findings do not
// leak headers into the render.
func collectJSToolingLines(clean string) ([]string, []shared.PriorityLine, []string) {
	collector := &jsToolingCollector{seen: map[string]struct{}{}}
	for _, line := range nonEmptyLines(clean) {
		collector.ingest(strings.TrimSpace(line))
	}
	return uniqueStrings(shared.FoldConsecutiveLines(collector.critical)),
		collector.grouped,
		uniqueStrings(shared.FoldConsecutiveLines(collector.summaries))
}

type jsToolingCollector struct {
	critical      []string
	grouped       []shared.PriorityLine
	summaries     []string
	pendingHeader string
	seen          map[string]struct{}
}

func (c *jsToolingCollector) ingest(trimmed string) {
	switch {
	case isJSRegistryErrorLine(trimmed):
		c.pendingHeader = ""
		c.critical = append(c.critical, clip(decodeRegistrySpec(trimmed), 200))
	case isJSToolingFileHeader(trimmed):
		c.pendingHeader = trimmed
	case isJSToolingSummaryLine(trimmed):
		c.pendingHeader = ""
		c.summaries = append(c.summaries, clip(trimmed, 160))
	case isJSToolingMessageLine(trimmed):
		c.ingestMessage(trimmed)
	}
}

// ingestMessage records a finding; the first finding under a pending file
// header pulls the header in and shares its group-lead tier.
func (c *jsToolingCollector) ingestMessage(trimmed string) {
	tier := jsToolingMessageTier(trimmed)
	if c.pendingHeader != "" {
		c.appendGrouped(clip(c.pendingHeader, 160), jsToolingTierGroupLead)
		c.pendingHeader = ""
		tier = jsToolingTierGroupLead
	}
	c.appendGrouped(clip(trimmed, 160), tier)
}

func (c *jsToolingCollector) appendGrouped(text string, tier int) {
	if _, dup := c.seen[text]; dup {
		return
	}
	c.seen[text] = struct{}{}
	c.grouped = append(c.grouped, shared.PriorityLine{Text: text, Tier: tier})
}

// isJSToolingFileHeader recognizes the bare file-path group headers linters
// print above per-line findings (eslint's stylish format, hyperlink-wrapped
// until ANSI/OSC-8 stripping). Losing the header loses the only line naming
// the file its findings belong to.
func isJSToolingFileHeader(line string) bool {
	if line == "" || strings.ContainsAny(line, " \t") || strings.Contains(line, "://") {
		return false
	}
	for _, ext := range []string{".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs", ".vue", ".svelte"} {
		if strings.HasSuffix(line, ext) {
			return true
		}
	}
	return false
}

func isJSToolingSummaryLine(line string) bool {
	for _, prefix := range []string{
		"npm ERR!", "ERR_PNPM_", " ERR_PNPM_", "error Command", "vite v",
		"transforming", "turbo ", "Failed projects:", "Failed tasks:",
		"warning", "Warnings:",
	} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	for _, marker := range []string{"Tasks:", "Failed Tasks:", "Nx read the output", "cache bypass"} {
		if strings.Contains(line, marker) {
			return true
		}
	}
	return false
}

func isJSToolingMessageLine(line string) bool {
	for _, marker := range []string{
		"error", "Error:", "failed",
		".ts:", ".mts:", ".cts:", ".tsx:", ".js:", ".mjs:", ".cjs:", ".jsx:", ".vue:", ".svelte:",
		"ELIFECYCLE", "Cannot find module", "RollupError", "Type error:", "esbuild", "TS",
	} {
		if strings.Contains(line, marker) {
			return true
		}
	}
	return strings.Contains(line, "Found ") && strings.Contains(line, "error")
}

// jsToolingMessageTier ranks findings: error-bearing lines outrank the rest,
// so a tight budget keeps every reported error before any secondary detail.
func jsToolingMessageTier(line string) int {
	lower := strings.ToLower(line)
	if strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "cannot find module") {
		return jsToolingTierError
	}
	return jsToolingTierDetail
}

// isJSRegistryErrorLine recognizes package-manager registry failures whose
// package spec and error code must survive the render: pnpm fetch errors,
// npm 404/resolution failures, and yarn resolution errors.
func isJSRegistryErrorLine(line string) bool {
	if strings.HasPrefix(line, "ERR_PNPM_") {
		return true
	}
	if strings.HasPrefix(line, "npm ERR!") || strings.HasPrefix(line, "npm error") {
		for _, marker := range []string{"404", "E404", "ERESOLVE", "ENOTFOUND", "ETARGET", "registry"} {
			if strings.Contains(line, marker) {
				return true
			}
		}
		return false
	}
	for _, marker := range []string{
		"is not in the npm registry",
		"is not in this registry",
		"No authorization header was set",
		"Couldn't find package",
		"No matching version found for",
	} {
		if strings.Contains(line, marker) {
			return true
		}
	}
	return false
}

// decodeRegistrySpec rewrites URL-encoded scoped package specs
// ("@acme%2Fdesign-tokens") back to their plain form so the failing package
// name stays greppable in the render.
func decodeRegistrySpec(line string) string {
	if !strings.Contains(line, "%") {
		return line
	}
	replacer := strings.NewReplacer("%2F", "/", "%2f", "/", "%40", "@")
	return replacer.Replace(line)
}
