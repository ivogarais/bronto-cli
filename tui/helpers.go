package tui

import "strings"

func isDangerLabel(label string) bool {
	s := strings.ToLower(strings.TrimSpace(label))
	return strings.Contains(s, "critical") ||
		strings.Contains(s, "fatal") ||
		strings.Contains(s, "panic") ||
		strings.Contains(s, "sev1") ||
		strings.Contains(s, "p0") ||
		strings.Contains(s, "error")
}

func barGapForDensity(density string) int {
	if density == "compact" {
		return 0
	}
	return 1
}

func splitByWeights(total, count int, weights []int) []int {
	if count <= 0 {
		return nil
	}
	if total < count {
		total = count
	}

	normalized := make([]int, count)
	if len(weights) != count {
		for i := range normalized {
			normalized[i] = 1
		}
	} else {
		for i, w := range weights {
			if w <= 0 {
				normalized[i] = 1
				continue
			}
			normalized[i] = w
		}
	}

	weightSum := 0
	for _, w := range normalized {
		weightSum += w
	}
	if weightSum <= 0 {
		return splitEven(total, count)
	}

	parts := make([]int, count)
	assigned := 0
	for i, w := range normalized {
		parts[i] = total * w / weightSum
		assigned += parts[i]
	}

	rem := total - assigned
	for i := 0; i < count && rem > 0; i++ {
		parts[i]++
		rem--
	}

	for i := range parts {
		if parts[i] < 1 {
			parts[i] = 1
		}
	}
	return parts
}

func splitEven(total, count int) []int {
	if count <= 0 {
		return nil
	}
	if total < count {
		total = count
	}

	base := total / count
	rem := total % count
	parts := make([]int, count)
	for i := range parts {
		parts[i] = base
		if i < rem {
			parts[i]++
		}
	}
	return parts
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
