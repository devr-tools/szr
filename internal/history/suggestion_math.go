package history

import "sort"

func percentileInts(values []int, target int) int {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int(nil), values...)
	sort.Ints(sorted)
	if target <= 0 {
		return sorted[0]
	}
	if target >= 100 {
		return sorted[len(sorted)-1]
	}
	index := (len(sorted)*target + 99) / 100
	if index <= 0 {
		index = 1
	}
	if index > len(sorted) {
		index = len(sorted)
	}
	return sorted[index-1]
}

func estimateBudgetLines(maxBytes, maxTokens int) int {
	linesByBytes := scaleIntCeil(maxBytes, 1, 48)
	linesByTokens := scaleIntCeil(maxTokens, 1, 12)
	lines := maxInt(linesByBytes, linesByTokens)
	if lines < 3 {
		lines = 3
	}
	if lines > 40 {
		lines = 40
	}
	return lines
}

func suggestionConfidence(samples int) string {
	switch {
	case samples >= 6:
		return "high"
	case samples >= 4:
		return "medium"
	default:
		return "low"
	}
}

func scaleIntCeil(value, num, den int) int {
	if value <= 0 || num <= 0 || den <= 0 {
		return 0
	}
	return (value*num + den - 1) / den
}

func maxInt(values ...int) int {
	best := 0
	for i, value := range values {
		if i == 0 || value > best {
			best = value
		}
	}
	return best
}
