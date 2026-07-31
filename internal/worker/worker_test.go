package worker

import (
	"testing"
	"time"
	"tracemind/internal/models"

	"github.com/stretchr/testify/assert"
)

func TestGroupBySourceAndWindow_SplitsBySourceEnvAndGap(t *testing.T) {
	t.Parallel()

	base := time.Now().UTC()
	signals := []models.Signal{
		{ID: "a1", Source: "svc-a", Env: "prod", Timestamp: base, Severity: 2},
		{ID: "a2", Source: "svc-a", Env: "prod", Timestamp: base.Add(10 * time.Second), Severity: 2},
		{ID: "a3", Source: "svc-a", Env: "prod", Timestamp: base.Add(2 * time.Minute), Severity: 4},
		{ID: "b1", Source: "svc-b", Env: "prod", Timestamp: base.Add(5 * time.Second), Severity: 3},
	}

	groups := groupBySourceAndWindow(signals, 30*time.Second)
	assert.Len(t, groups, 3)
}
