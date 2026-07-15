package history

// TokenMemo caches EstimateTokens results by content for the lifetime of one
// command's render pipeline. The sequential render passes re-estimate the
// same raw and rendered strings; a content-keyed lookup turns each repeat
// into a map probe instead of a full byte scan. A nil memo estimates
// directly, so call sites without a pipeline context work unchanged.
type TokenMemo map[string]int

// Estimate returns EstimateTokens(text), memoized by content.
func (m TokenMemo) Estimate(text string) int {
	if m == nil {
		return EstimateTokens(text)
	}
	if tokens, ok := m[text]; ok {
		return tokens
	}
	tokens := EstimateTokens(text)
	m[text] = tokens
	return tokens
}
