package models

import "time"

type RuleMatchType string

const (
	MatchTypeSingle      RuleMatchType = "single"
	MatchTypeCorrelation RuleMatchType = "correlation"
)

type MessageMatchType string

const (
	MessageMatchExact    MessageMatchType = "exact"
	MessageMatchContains MessageMatchType = "contains"
	MessageMatchRegex    MessageMatchType = "regex"
)

// Signal represents an incoming event/signal
type Signal struct {
	ID        string                 `json:"id,omitempty"`
	EventType string                 `json:"eventType"`
	Source    string                 `json:"source"`
	Env       string                 `json:"environment"`
	Timestamp time.Time              `json:"timestamp"`
	Severity  int                    `json:"severity"`
	Message   string                 `json:"message,omitempty"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Metadata  map[string]string      `json:"metadata,omitempty"`
}

// IngestRequest is the POST /api/ingest body
type IngestRequest struct {
	SourceContext string   `json:"sourceContext"`
	Signals       []Signal `json:"signals"`
}

// IngestResponse
type IngestResponse struct {
	IngestionID   string   `json:"ingestionId"`
	AcceptedCount int      `json:"acceptedCount"`
	RejectedCount int      `json:"rejectedCount"`
	Errors        []string `json:"errors,omitempty"`
}

// Incident represents a correlated incident
type Incident struct {
	ID               string    `json:"id"`
	Title            string    `json:"title"`
	Status           string    `json:"status"`
	Severity         int       `json:"severity"`
	ImpactedServices []string  `json:"impactedServices,omitempty"`
	Environments     []string  `json:"environments,omitempty"`
	SignalIDs        []string  `json:"signalIds,omitempty"`
	AnalysisSummary  string    `json:"analysisSummary,omitempty"`
	Recommendations  []string  `json:"recommendations,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// AnalysisResult placeholder
type AnalysisResult struct {
	IncidentID      string    `json:"incidentId"`
	Hypotheses      []string  `json:"rootCauseHypotheses"`
	Confidence      []float64 `json:"confidenceScores"`
	Recommendations []string  `json:"recommendedActions"`
	Timestamp       time.Time `json:"analysisTimestamp"`
	Source          string    `json:"source"`
}

type PayloadCondition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// AnalysisRule stores a deterministic analysis rule configuration.
type AnalysisRule struct {
	ID                 string                `json:"id"`
	Name               string                `json:"name"`
	Description        string                `json:"description,omitempty"`
	Confidence         float64               `json:"confidence,omitempty"`
	Priority           int                   `json:"priority"`
	Enabled            bool                  `json:"enabled"`
	MatchType          RuleMatchType         `json:"matchType"`
	HypothesisTemplate string                `json:"hypothesisTemplate"`
	Recommendations    []string              `json:"recommendations,omitempty"`
	Patterns           []AnalysisRulePattern `json:"patterns,omitempty"`
	Version            int                   `json:"version"`
	CreatedAt          time.Time             `json:"createdAt"`
	UpdatedAt          time.Time             `json:"updatedAt"`
}

// AnalysisRulePattern stores match patterns belonging to an analysis rule.
type AnalysisRulePattern struct {
	ID                string             `json:"id"`
	RuleID            string             `json:"ruleId"`
	EventType         string             `json:"eventType,omitempty"` //database, queue, deployment, health, authentication
	Source            string             `json:"source,omitempty"`    //where the signal came from.
	Environment       string             `json:"environment,omitempty"`
	SeverityMin       *int               `json:"severityMin,omitempty"` //Only match signals whose severity is at least this value.
	MessageMatchType  MessageMatchType   `json:"messageMatchType,omitempty"`
	MessagePattern    string             `json:"messagePattern,omitempty"`
	PayloadConditions []PayloadCondition `json:"payloadConditions,omitempty"`
	VariableMappings  map[string]string  `json:"variableMappings,omitempty"`
	CreatedAt         time.Time          `json:"createdAt"`
	UpdatedAt         time.Time          `json:"updatedAt"`
}

/*
The pointer lets you distinguish between:
"No minimum severity specified"
"Minimum severity is 0"
*/
