package builtin

import (
	"fmt"
	"strconv"
	"strings"
)

func getRequiredString(input map[string]any, key string) (string, error) {
	return getRequiredStringWithPolicy(input, key, false)
}

func getRequiredStringAllowEmpty(input map[string]any, key string) (string, error) {
	return getRequiredStringWithPolicy(input, key, true)
}

func getRequiredStringWithPolicy(input map[string]any, key string, allowEmpty bool) (string, error) {
	raw, ok := input[key]
	if !ok {
		return "", fmt.Errorf("missing required argument '%s'", key)
	}
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("argument '%s' must be a string", key)
	}
	if !allowEmpty && strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("argument '%s' cannot be empty", key)
	}
	return s, nil
}

func getOptionalInt(input map[string]any, key string, fallback int) (int, error) {
	raw, ok := input[key]
	if !ok {
		return fallback, nil
	}

	switch v := raw.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return fallback, nil
		}
		parsed, err := strconv.Atoi(trimmed)
		if err != nil {
			return 0, fmt.Errorf("argument '%s' must be an integer", key)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("argument '%s' must be an integer", key)
	}
}
