package analysis

import (
	"testing"
	"tracemind/internal/models"
	"tracemind/internal/store"

	"github.com/stretchr/testify/assert"
)

func TestAnalyzerReturnsResultAndError(t *testing.T) {
	var analyze func(models.Incident, []models.Signal, store.PostgresStore) (models.AnalysisResult, error) = NewRuleEngine().Analyze
	if analyze == nil {
		t.Fatal("expected analyzer function")
	}
}

func TestEvaluateRuleAgainstEvidence_SingleAndCorrelation(t *testing.T) {
	t.Parallel()

	evidence := []models.Signal{
		{Message: "db timeout", Payload: map[string]interface{}{"durationMs": 650.0}},
		{Message: "retry succeeded", Payload: map[string]interface{}{"attempt": 2}},
	}

	singleRule := models.AnalysisRule{
		MatchType: models.MatchTypeSingle,
		Patterns: []models.AnalysisRulePattern{{
			MessageMatchType: models.MessageMatchContains,
			MessagePattern:   "timeout",
		}},
	}

	correlationRule := models.AnalysisRule{
		MatchType: models.MatchTypeCorrelation,
		Patterns: []models.AnalysisRulePattern{
			{MessageMatchType: models.MessageMatchContains, MessagePattern: "timeout"},
			{MessageMatchType: models.MessageMatchContains, MessagePattern: "retry"},
		},
	}

	correlationMiss := models.AnalysisRule{
		MatchType: models.MatchTypeCorrelation,
		Patterns: []models.AnalysisRulePattern{
			{MessageMatchType: models.MessageMatchContains, MessagePattern: "timeout"},
			{MessageMatchType: models.MessageMatchContains, MessagePattern: "cpu spike"},
		},
	}

	assert.True(t, evaluateRuleAgainstEvidence(evidence, singleRule))
	assert.True(t, evaluateRuleAgainstEvidence(evidence, correlationRule))
	assert.False(t, evaluateRuleAgainstEvidence(evidence, correlationMiss))
}

func TestMatchesMessage(t *testing.T) {
	t.Parallel()

	assert.True(t, matchesMessage("disk full", models.MessageMatchExact, "disk full"))
	assert.True(t, matchesMessage("disk nearly full", models.MessageMatchContains, "nearly"))
	assert.True(t, matchesMessage("timeout after 120ms", models.MessageMatchRegex, `timeout after \d+ms`))
	assert.False(t, matchesMessage("timeout", models.MessageMatchRegex, "["))
	assert.False(t, matchesMessage("timeout", "unknown", "timeout"))
}

func TestMatchesPayloadCondition(t *testing.T) {
	t.Parallel()

	payload := map[string]interface{}{
		"service":    "checkout-api",
		"durationMs": 480.0,
		"attempt":    2,
	}

	assert.True(t, matchesPayloadCondition(payload, models.PayloadCondition{Field: "service", Operator: "equals", Value: "checkout-api"}))
	assert.True(t, matchesPayloadCondition(payload, models.PayloadCondition{Field: "service", Operator: "contains", Value: "checkout"}))
	assert.True(t, matchesPayloadCondition(payload, models.PayloadCondition{Field: "durationMs", Operator: "greater_than", Value: 300}))
	assert.True(t, matchesPayloadCondition(payload, models.PayloadCondition{Field: "durationMs", Operator: "less_than", Value: 1000}))
	assert.True(t, matchesPayloadCondition(payload, models.PayloadCondition{Field: "attempt", Operator: "greater_than_equal", Value: 2}))
	assert.True(t, matchesPayloadCondition(payload, models.PayloadCondition{Field: "attempt", Operator: "less_than_equal", Value: 2.0}))
	assert.False(t, matchesPayloadCondition(payload, models.PayloadCondition{Field: "missing", Operator: "equals", Value: "x"}))
	assert.False(t, matchesPayloadCondition(payload, models.PayloadCondition{Field: "attempt", Operator: "contains", Value: "2"}))
	assert.False(t, matchesPayloadCondition(payload, models.PayloadCondition{Field: "service", Operator: "greater_than", Value: 1}))
	assert.False(t, matchesPayloadCondition(payload, models.PayloadCondition{Field: "attempt", Operator: "unsupported", Value: 2}))
}

func TestConfidenceScoresAndDedupeStrings(t *testing.T) {
	t.Parallel()

	assert.Nil(t, confidenceScores(0, "rule-based"))
	ruleBased := confidenceScores(8, "rule-based")
	assert.Len(t, ruleBased, 8)
	assert.InDeltaSlice(t, []float64{0.83, 0.76, 0.69, 0.62, 0.55, 0.48, 0.41, 0.4}, ruleBased, 1e-9)

	hybrid := confidenceScores(2, "hybrid")
	assert.Len(t, hybrid, 2)
	assert.InDeltaSlice(t, []float64{0.62, 0.55}, hybrid, 1e-9)

	assert.Nil(t, dedupeStrings(nil))
	assert.Equal(t, []string{"a", "b", "c"}, dedupeStrings([]string{"a", "b", "a", "c", "b"}))
}
