package analysis

import (
	"strings"
	"tracemind/internal/models"
)

type ruleMatch struct {
	hypothesis      string
	recommendations []string
}

func detectDeploymentOutage(evidence []models.Signal) (ruleMatch, bool) {
	if !hasEventType(evidence, "deployment") {
		return ruleMatch{}, false
	}

	if !hasHealthDegradation(evidence) {
		return ruleMatch{}, false
	}

	return ruleMatch{
		hypothesis: "deployment regression likely caused service outage",
		recommendations: []string{
			"Validate rollout health checks and compare the current release against the previous revision.",
			"Rollback or disable the latest deployment while capturing failing request traces.",
		},
	}, true
}

func detectDatabaseFailure(evidence []models.Signal) (ruleMatch, bool) {
	hasDatabaseSignal := false
	hasDatabaseFailureText := false

	for _, signal := range evidence {
		if signal.EventType != "database" {
			continue
		}
		hasDatabaseSignal = true
		if signal.Severity >= 4 || messageContainsAny(signal.Message, "connection", "too many connections", "pool", "refused", "timeout") {
			hasDatabaseFailureText = true
		}
	}

	if !hasDatabaseSignal || !hasDatabaseFailureText {
		return ruleMatch{}, false
	}

	return ruleMatch{
		hypothesis: "database connectivity failure is driving service instability",
		recommendations: []string{
			"Inspect database connection pool saturation and active sessions.",
			"Scale database resources or tune pool limits before traffic recovers.",
		},
	}, true
}

func detectQueueBacklog(evidence []models.Signal) (ruleMatch, bool) {
	hasQueueSignal := false
	hasBacklogIndicator := false

	for _, signal := range evidence {
		if signal.EventType != "queue" {
			continue
		}
		hasQueueSignal = true
		if signal.Severity >= 4 || messageContainsAny(signal.Message, "backlog", "lag", "depth", "threshold", "stuck") {
			hasBacklogIndicator = true
		}
	}

	if !hasQueueSignal || !hasBacklogIndicator {
		return ruleMatch{}, false
	}

	if !hasHealthDegradation(evidence) {
		return ruleMatch{}, false
	}

	return ruleMatch{
		hypothesis: "queue backlog is degrading end-to-end processing health",
		recommendations: []string{
			"Inspect consumer throughput and retry rates to identify processing bottlenecks.",
			"Scale workers or tune visibility timeout before queue latency breaches SLOs.",
		},
	}, true
}

func hasEventType(evidence []models.Signal, eventType string) bool {
	for _, signal := range evidence {
		if signal.EventType == eventType {
			return true
		}
	}
	return false
}

func hasHealthDegradation(evidence []models.Signal) bool {
	for _, signal := range evidence {
		if signal.EventType != "health" {
			continue
		}
		if signal.Severity >= 4 || messageContainsAny(signal.Message, "timeout", "unavailable", "degraded", "error") {
			return true
		}
	}
	return false
}

func messageContainsAny(message string, terms ...string) bool {
	msg := strings.ToLower(message)
	for _, term := range terms {
		if strings.Contains(msg, strings.ToLower(term)) {
			return true
		}
	}
	return false
}
