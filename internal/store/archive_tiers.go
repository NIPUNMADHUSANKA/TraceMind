package store

import (
	"strings"
	"time"
)

type ArchiveTier string

const (
	ArchiveTierHot  ArchiveTier = "hot"
	ArchiveTierWarm ArchiveTier = "warm"
	ArchiveTierCold ArchiveTier = "cold"
)

type RetentionProfile struct {
	RawWindow           time.Duration
	NormalizedWindow    time.Duration
	LowSeveritySampling float64
}

func ResolveArchiveTier(age time.Duration) ArchiveTier {
	switch {
	case age <= 7*24*time.Hour:
		return ArchiveTierHot
	case age <= 30*24*time.Hour:
		return ArchiveTierWarm
	default:
		return ArchiveTierCold
	}
}

func RetentionProfileForEnvironment(env string) RetentionProfile {
	switch normalizeEnvironment(env) {
	case "prod", "production":
		return RetentionProfile{
			RawWindow:           30 * 24 * time.Hour,
			NormalizedWindow:    365 * 24 * time.Hour,
			LowSeveritySampling: 0.01,
		}
	case "staging", "stage":
		return RetentionProfile{
			RawWindow:           14 * 24 * time.Hour,
			NormalizedWindow:    90 * 24 * time.Hour,
			LowSeveritySampling: 0.01,
		}
	default:
		return RetentionProfile{
			RawWindow:           7 * 24 * time.Hour,
			NormalizedWindow:    30 * 24 * time.Hour,
			LowSeveritySampling: 0.01,
		}
	}
}

func normalizeEnvironment(env string) string {
	return strings.ToLower(strings.TrimSpace(env))
}
