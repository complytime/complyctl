// SPDX-License-Identifier: Apache-2.0

package jsonschema

import "fmt"

var knownKeys = map[string][]string{
	"": {"id", "evaluator-id", "version", "gemara", "schemas",
		"policies", "tests", "fixtures", "output"},
	"gemara":           {"source", "sources", "plain-http"},
	"gemara.sources[]": {"source", "plain-http"},
	"schemas[]":        {"platform", "source", "path"},
	"policies":         {"dir", "helpers"},
	"tests":            {"dir"},
	"fixtures":         {"dir"},
	"output":           {"dir"},
}

func checkUnknownKeys(m map[string]any) []string {
	var warnings []string
	walkUnknown(m, "", &warnings)
	return warnings
}

func walkUnknown(m map[string]any, scope string, warnings *[]string) {
	known, ok := knownKeys[scope]
	if !ok {
		return
	}

	knownSet := make(map[string]bool, len(known))
	for _, k := range known {
		knownSet[k] = true
	}

	for key, val := range m {
		if !knownSet[key] {
			*warnings = append(*warnings, formatUnknownWarning(key, known))
			continue
		}

		switch v := val.(type) {
		case map[string]any:
			childScope := key
			if scope != "" {
				childScope = scope + "." + key
			}
			walkUnknown(v, childScope, warnings)
		case []any:
			arrayScope := key + "[]"
			if scope != "" {
				arrayScope = scope + "." + key + "[]"
			}
			for _, item := range v {
				if itemMap, ok := item.(map[string]any); ok {
					walkUnknown(itemMap, arrayScope, warnings)
				}
			}
		}
	}
}

func formatUnknownWarning(key string, known []string) string {
	best := ""
	bestDist := 3
	for _, k := range known {
		d := levenshtein(key, k)
		if d < bestDist {
			bestDist = d
			best = k
		}
	}
	if best != "" {
		return fmt.Sprintf("unknown field %q (did you mean %q?)", key, best)
	}
	return fmt.Sprintf("unknown field %q", key)
}

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			curr[j] = min(del, min(ins, sub))
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}
