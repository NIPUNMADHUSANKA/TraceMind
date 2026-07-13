package analysis

import (
	"time"
	"tracemind/internal/models"
)

// Analyzer produces hypothesis and recommendation output for an incident.
type Analyzer interface {
	Analyze(incident models.Incident, evidence []models.Signal) models.AnalysisResult
}

type ruleEngine struct{}

func NewRuleEngine() Analyzer {
	return &ruleEngine{}
}

func (e *ruleEngine) Analyze(incident models.Incident, evidence []models.Signal) models.AnalysisResult {
	hypotheses := make([]string, 0, 3)
	recommendations := make([]string, 0, 6)

	if match, ok := detectDeploymentOutage(evidence); ok {
		hypotheses = append(hypotheses, match.hypothesis)
		recommendations = append(recommendations, match.recommendations...)
	}
	if match, ok := detectDatabaseFailure(evidence); ok {
		hypotheses = append(hypotheses, match.hypothesis)
		recommendations = append(recommendations, match.recommendations...)
	}
	if match, ok := detectQueueBacklog(evidence); ok {
		hypotheses = append(hypotheses, match.hypothesis)
		recommendations = append(recommendations, match.recommendations...)
	}

	source := "rule-based"
	if len(hypotheses) == 0 {
		hypotheses = append(hypotheses, "insufficient deterministic evidence")
		recommendations = append(recommendations,
			"Collect additional traces and infrastructure metrics for this window.",
			"Escalate to hybrid analysis with service owner context.",
		)
		source = "hybrid"
	}

	return models.AnalysisResult{
		IncidentID:      incident.ID,
		Hypotheses:      dedupeStrings(hypotheses),
		Confidence:      confidenceScores(len(hypotheses), source),
		Recommendations: dedupeStrings(recommendations),
		Timestamp:       time.Now().UTC(),
		Source:          source,
	}
}

func confidenceScores(hypothesisCount int, source string) []float64 {
	if hypothesisCount <= 0 {
		return nil
	}

	base := 0.62
	if source == "rule-based" {
		base = 0.83
	}

	scores := make([]float64, 0, hypothesisCount)
	for i := 0; i < hypothesisCount; i++ {
		score := base - (0.07 * float64(i))
		if score < 0.4 {
			score = 0.4
		}
		scores = append(scores, score)
	}
	return scores
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result
}
