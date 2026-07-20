package analysis

import (
	"regexp"
	"strings"
	"time"
	"tracemind/internal/models"
	"tracemind/internal/store"
)

// Analyzer produces hypothesis and recommendation output for an incident.
type Analyzer interface {
	Analyze(incident models.Incident, evidence []models.Signal, s store.PostgresStore) models.AnalysisResult
}

type ruleEngine struct{}

func NewRuleEngine() Analyzer {
	return &ruleEngine{}
}

func (e *ruleEngine) Analyze(incident models.Incident, evidence []models.Signal, s store.PostgresStore) models.AnalysisResult {

	var source, environment string
	var eventType []string
	if len(evidence) > 0 {
		source = evidence[0].Source
		environment = evidence[0].Env
	}

	for _, signal := range evidence {
		eventType = append(eventType, signal.EventType)
	}

	rules, err := s.GetEnabledAnalysisRulesByPattern(eventType, source, environment)
	if err != nil {
		// Log error and proceed with empty rules
		hypotheses := []string{}
		recommendations := []string{}
		hypotheses = append(hypotheses, "insufficient deterministic evidence")
		recommendations = append(recommendations,
			"Collect additional traces and infrastructure metrics for this window.",
			"Escalate to hybrid analysis with service owner context.",
		)
		return models.AnalysisResult{
			IncidentID:      incident.ID,
			Hypotheses:      dedupeStrings(hypotheses),
			Confidence:      confidenceScores(len(hypotheses), "hybrid"),
			Recommendations: dedupeStrings(recommendations),
			Timestamp:       time.Now().UTC(),
			Source:          "hybrid",
		}
	}

	hypotheses := []string{}
	recommendations := []string{}

	for _, rule := range rules {
		if len(rule.Patterns) == 0 {
			continue
		}

		ruleMatched := evaluateRuleAgainstEvidence(evidence, rule)
		if ruleMatched {
			hypotheses = append(hypotheses, rule.HypothesisTemplate)
			recommendations = append(recommendations, rule.Recommendations...)
		}
	}

	analysisSource := "rule-based"
	if len(hypotheses) == 0 {
		hypotheses = append(hypotheses, "insufficient deterministic evidence")
		recommendations = append(recommendations,
			"Collect additional traces and infrastructure metrics for this window.",
			"Escalate to hybrid analysis with service owner context.",
		)
		analysisSource = "hybrid"
	}

	return models.AnalysisResult{
		IncidentID:      incident.ID,
		Hypotheses:      dedupeStrings(hypotheses),
		Confidence:      confidenceScores(len(hypotheses), analysisSource),
		Recommendations: dedupeStrings(recommendations),
		Timestamp:       time.Now().UTC(),
		Source:          analysisSource,
	}
}

// evaluateRuleAgainstEvidence checks if a rule matches the evidence based on MatchType.
// For MatchTypeSingle: returns true if ANY pattern matches any signal.
// For MatchTypeCorrelation: returns true if ALL patterns match (across any signals).
func evaluateRuleAgainstEvidence(evidence []models.Signal, rule models.AnalysisRule) bool {
	if rule.MatchType == models.MatchTypeSingle {
		// Single: trigger if ANY pattern matches ANY signal
		for _, signal := range evidence {
			for _, pattern := range rule.Patterns {
				if matchesPattern(signal, pattern) {
					return true
				}
			}
		}
		return false
	} else {
		// Correlation: trigger if ALL patterns match (each pattern needs at least one matching signal)
		for _, pattern := range rule.Patterns {
			patternMatched := false
			for _, signal := range evidence {
				if matchesPattern(signal, pattern) {
					patternMatched = true
					break
				}
			}
			if !patternMatched {
				return false
			}
		}
		return true
	}
}

// matchesPattern checks if a signal matches a pattern's message and payload conditions.
func matchesPattern(signal models.Signal, pattern models.AnalysisRulePattern) bool {
	// Check message match
	if !matchesMessage(signal.Message, pattern.MessageMatchType, pattern.MessagePattern) {
		return false
	}

	// Check payload conditions
	for _, condition := range pattern.PayloadConditions {
		if !matchesPayloadCondition(signal.Payload, condition) {
			return false
		}
	}

	return true
}

// matchesMessage checks if a message matches the pattern based on match type.
func matchesMessage(message string, matchType models.MessageMatchType, pattern string) bool {
	switch matchType {
	case models.MessageMatchExact:
		return message == pattern
	case models.MessageMatchContains:
		return strings.Contains(message, pattern)
	case models.MessageMatchRegex:
		match, err := regexp.MatchString(pattern, message)
		if err != nil {
			return false
		}
		return match
	default:
		return false
	}
}

// matchesPayloadCondition checks if a payload field matches a condition.
func matchesPayloadCondition(payload map[string]interface{}, condition models.PayloadCondition) bool {
	fieldValue, exists := payload[condition.Field]
	if !exists {
		return false
	}

	switch condition.Operator {
	case "equals":
		return fieldValue == condition.Value
	case "contains":
		str, ok := fieldValue.(string)
		if !ok {
			return false
		}
		condValue, ok := condition.Value.(string)
		if !ok {
			return false
		}
		return strings.Contains(str, condValue)
	case "greater_than":
		return compareNumeric(fieldValue, condition.Value, ">")
	case "less_than":
		return compareNumeric(fieldValue, condition.Value, "<")
	case "greater_than_equal":
		return compareNumeric(fieldValue, condition.Value, ">=")
	case "less_than_equal":
		return compareNumeric(fieldValue, condition.Value, "<=")
	default:
		return false
	}
}

// compareNumeric safely compares two values as numbers.
func compareNumeric(fieldValue interface{}, condValue interface{}, operator string) bool {
	// Try to extract numeric values
	var field, cond float64
	var ok bool

	if field, ok = toFloat64(fieldValue); !ok {
		return false
	}
	if cond, ok = toFloat64(condValue); !ok {
		return false
	}

	switch operator {
	case ">":
		return field > cond
	case "<":
		return field < cond
	case ">=":
		return field >= cond
	case "<=":
		return field <= cond
	default:
		return false
	}
}

// toFloat64 safely converts a value to float64.
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
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
