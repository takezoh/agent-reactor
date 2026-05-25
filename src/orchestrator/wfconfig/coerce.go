package wfconfig

import (
	"fmt"
	"strconv"
	"strings"
)

// coerceInt converts v to int. Accepts int, int64, float64 (whole), or string.
func coerceInt(v any) (int, error) {
	switch t := v.(type) {
	case int:
		return t, nil
	case int64:
		return int(t), nil
	case float64:
		if t != float64(int(t)) {
			return 0, fmt.Errorf("%w: float %v is not a whole number", ErrConfigCoerce, t)
		}
		return int(t), nil
	case string:
		n, err := strconv.Atoi(t)
		if err != nil {
			return 0, fmt.Errorf("%w: cannot parse %q as int", ErrConfigCoerce, t)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("%w: unexpected type %T for int field", ErrConfigCoerce, v)
	}
}

// coerceString converts v to string. Only accepts string.
func coerceString(v any) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%w: expected string, got %T", ErrConfigCoerce, v)
	}
	return s, nil
}

// coerceStringSlice converts []any to []string.
func coerceStringSlice(v any) ([]string, error) {
	raw, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: expected list, got %T", ErrConfigCoerce, v)
	}
	out := make([]string, 0, len(raw))
	for i, elem := range raw {
		s, ok := elem.(string)
		if !ok {
			return nil, fmt.Errorf("%w: list element %d is %T, not string", ErrConfigCoerce, i, elem)
		}
		out = append(out, s)
	}
	return out, nil
}

// coercePerStateMap converts map[string]any to map[string]int with lowercase keys.
// Non-positive values and entries that fail int coercion are silently dropped (SPEC §5.3.5).
func coercePerStateMap(v any) (map[string]int, error) {
	raw, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: expected map, got %T", ErrConfigCoerce, v)
	}
	out := make(map[string]int, len(raw))
	for k, val := range raw {
		n, err := coerceInt(val)
		if err != nil || n <= 0 {
			continue
		}
		out[strings.ToLower(k)] = n
	}
	return out, nil
}
