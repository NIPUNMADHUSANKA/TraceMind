package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResolveArchiveTier(t *testing.T) {
	t.Parallel()

	require.Equal(t, ArchiveTierHot, ResolveArchiveTier(6*time.Hour))
	require.Equal(t, ArchiveTierWarm, ResolveArchiveTier(20*24*time.Hour))
	require.Equal(t, ArchiveTierCold, ResolveArchiveTier(120*24*time.Hour))
}

func TestRetentionProfileForEnvironment_ProdDefaults(t *testing.T) {
	t.Parallel()

	profile := RetentionProfileForEnvironment("prod")
	require.Equal(t, 30*24*time.Hour, profile.RawWindow)
	require.Equal(t, 365*24*time.Hour, profile.NormalizedWindow)
	require.Equal(t, 0.01, profile.LowSeveritySampling)
}
