package worker

import (
	"testing"
	"time"
	"tracemind/internal/models"
	"tracemind/internal/queue"
	"tracemind/internal/store"

	"github.com/stretchr/testify/require"
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
	require.Len(t, groups, 3)
}

func TestProcessJob_CreatesIncidentForHighSeverityGroup(t *testing.T) {
	t.Parallel()

	s := store.NewStore()
	base := time.Now().UTC()
	job := ingestionJobForTest([]models.Signal{
		{ID: "h1", EventType: "log", Source: "svc-a", Env: "prod", Timestamp: base, Severity: 5},
		{ID: "h2", EventType: "log", Source: "svc-a", Env: "prod", Timestamp: base.Add(5 * time.Second), Severity: 4},
	})

	processJob(job, s)

	incidents := s.ListIncidents()
	require.Len(t, incidents, 1)
	require.ElementsMatch(t, []string{"h1", "h2"}, incidents[0].SignalIDs)
	require.Equal(t, 5, incidents[0].Severity)
	require.Equal(t, []string{"svc-a"}, incidents[0].ImpactedServices)
	require.Equal(t, []string{"prod"}, incidents[0].Environments)
}

func TestProcessJob_MergesIntoExistingIncident_WhenRelated(t *testing.T) {
	t.Parallel()

	s := store.NewStore()
	base := time.Now().UTC()
	s.SaveIncident(models.Incident{
		ID:               "inc-existing",
		Title:            "Auto-generated incident",
		Status:           "open",
		Severity:         4,
		SignalIDs:        []string{"prev"},
		ImpactedServices: []string{"svc-a"},
		Environments:     []string{"prod"},
		UpdatedAt:        base,
	})

	job := ingestionJobForTest([]models.Signal{
		{ID: "n1", EventType: "log", Source: "svc-a", Env: "prod", Timestamp: base.Add(10 * time.Second), Severity: 5},
	})

	processJob(job, s)

	inc, ok := s.GetIncident("inc-existing")
	require.True(t, ok)
	require.ElementsMatch(t, []string{"prev", "n1"}, inc.SignalIDs)
	require.Equal(t, 5, inc.Severity)
}

func ingestionJobForTest(signals []models.Signal) queueIngestionJobAlias {
	return queueIngestionJobAlias{Signals: signals}
}

type queueIngestionJobAlias = queue.IngestionJob
