package testkit

import (
	"fmt"
	"strings"
)

// traverse walks a nested JSON object by dot-path.
//
// Path syntax:
//
//	"user.name"     — nested map lookup
//	"items.0.sku"   — array index by decimal
//	""              — returns the map itself
//
// A missing key or an out-of-range index returns nil rather than an
// error, so HasField can compare nil to nil when a caller asserts on
// an absent field deliberately. ExpectArrayLen relies on the same
// behavior.
func traverse(m map[string]any, path string) any {
	if path == "" {
		return m
	}
	if val, ok := m[path]; ok {
		return val
	}

	parts := strings.Split(path, ".")
	var cur any = m
	for _, p := range parts {
		switch target := cur.(type) {
		case map[string]any:
			cur = target[p]
		case []any:
			idx, err := parseIndex(p, len(target))
			if err != nil {
				return nil
			}
			cur = target[idx]
		default:
			return nil
		}
	}
	return cur
}

func parseIndex(p string, length int) (int, error) {
	var idx int
	if _, err := fmt.Sscanf(p, "%d", &idx); err != nil {
		return 0, err
	}
	if idx < 0 || idx >= length {
		return 0, fmt.Errorf("index %d out of range [0,%d)", idx, length)
	}
	return idx, nil
}
