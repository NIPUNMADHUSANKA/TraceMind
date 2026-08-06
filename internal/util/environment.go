package util

import (
	"fmt"
	"strings"
)

func FormatEnvironment(env string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(env))

	switch normalized {
	case "prod", "production":
		return "production", nil
	case "staging", "stage":
		return "staging", nil
	case "dev", "development":
		return "dev", nil
	}

	return normalized, fmt.Errorf("invalid environment: %s", normalized)
}
