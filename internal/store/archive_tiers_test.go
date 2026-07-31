package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetentionProfileForEnvironment_ProdDefaults(t *testing.T) {
	t.Parallel()

	profile := RetentionProfileForEnvironment("prod")
	assert.Equal(t, 30*24*time.Hour, profile.RawWindow)
	assert.Equal(t, 365*24*time.Hour, profile.NormalizedWindow)
}
