package analysis

import (
	"strings"
	"testing"
	"time"
	"tracemind/internal/models"

	"github.com/stretchr/testify/require"
)

func TestRuleEngine_DatabaseFailurePattern(t *testing.T) {
	t.Parallel()

	engine := NewRuleEngine()
	result := engine.Analyze(models.Incident{ID: "inc-db"}, []models.Signal{
		{EventType: "database", Severity: 5, Message: "too many connections"},
		{EventType: "health", Severity: 4, Message: "service timeout"},
	})

	require.Contains(t, strings.Join(result.Hypotheses, " "), "database")
	require.NotEmpty(t, result.Recommendations)
	require.Equal(t, "rule-based", result.Source)
}

func TestRuleEngine_DeploymentOutagePattern(t *testing.T) {
	t.Parallel()

	engine := NewRuleEngine()
	result := engine.Analyze(models.Incident{ID: "inc-deploy"}, []models.Signal{
		{EventType: "deployment", Severity: 4, Message: "release completed 3m ago"},
		{EventType: "health", Severity: 5, Message: "service unavailable"},
	})

	require.Contains(t, strings.Join(result.Hypotheses, " "), "deployment")
	require.NotEmpty(t, result.Recommendations)
	require.Equal(t, "rule-based", result.Source)
}

func TestRuleEngine_QueueBacklogPattern(t *testing.T) {
	t.Parallel()

	engine := NewRuleEngine()
	result := engine.Analyze(models.Incident{ID: "inc-queue"}, []models.Signal{
		{EventType: "queue", Severity: 4, Message: "queue backlog crossed threshold"},
		{EventType: "health", Severity: 4, Message: "latency elevated"},
	})

	require.Contains(t, strings.Join(result.Hypotheses, " "), "queue")
	require.NotEmpty(t, result.Recommendations)
	require.Equal(t, "rule-based", result.Source)
}

func TestRuleEngine_HybridFallbackWhenNoDeterministicEvidence(t *testing.T) {
	t.Parallel()

	engine := NewRuleEngine()
	result := engine.Analyze(models.Incident{ID: "inc-hybrid"}, []models.Signal{{
		EventType: "log",
		Severity:  2,
		Message:   "minor warning",
	}})

	require.Equal(t, "hybrid", result.Source)
	require.Equal(t, []string{"insufficient deterministic evidence"}, result.Hypotheses)
	require.NotEmpty(t, result.Recommendations)
	require.False(t, result.Timestamp.IsZero())
}

func TestRuleEngine_AssignsIncidentIDAndTimestamp(t *testing.T) {
	t.Parallel()

	engine := NewRuleEngine()
	result := engine.Analyze(models.Incident{ID: "inc-meta"}, []models.Signal{
		{EventType: "database", Severity: 5, Message: "connection refused"},
	})

	require.Equal(t, "inc-meta", result.IncidentID)
	require.WithinDuration(t, time.Now().UTC(), result.Timestamp, 2*time.Second)
}
