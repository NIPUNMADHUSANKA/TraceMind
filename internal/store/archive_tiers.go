package store

import (
	"strings"
	"time"
)

type RetentionProfile struct {
	RawWindow        time.Duration
	NormalizedWindow time.Duration
}

func RetentionProfileForEnvironment(env string) RetentionProfile {
	switch normalizeEnvironment(env) {
	case "prod", "production":
		return RetentionProfile{
			RawWindow:        30 * 24 * time.Hour,
			NormalizedWindow: 365 * 24 * time.Hour,
		}
	case "staging", "stage":
		return RetentionProfile{
			RawWindow:        14 * 24 * time.Hour,
			NormalizedWindow: 90 * 24 * time.Hour,
		}
	default:
		return RetentionProfile{
			RawWindow:        7 * 24 * time.Hour,
			NormalizedWindow: 30 * 24 * time.Hour,
		}
	}
}

func normalizeEnvironment(env string) string {
	return strings.ToLower(strings.TrimSpace(env))
}
