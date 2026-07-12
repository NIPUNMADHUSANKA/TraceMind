package models

import "time"

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
