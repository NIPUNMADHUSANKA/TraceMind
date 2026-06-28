package api

import (
	"fmt"
	"time"
	"tracemind/internal/models"
	"tracemind/internal/queue"
	"tracemind/internal/store"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

var allowedEventTypes = map[string]bool{
	"log":        true,
	"deployment": true,
	"database":   true,
	"queue":      true,
	"health":     true,
}

type ingestRequestInput struct {
	SourceContext string              `json:"sourceContext"`
	Signals       []ingestSignalInput `json:"signals"`
}

type ingestSignalInput struct {
	ID        string                 `json:"id,omitempty"`
	EventType string                 `json:"eventType"`
	Source    string                 `json:"source"`
	Env       string                 `json:"environment"`
	Timestamp *string                `json:"timestamp"`
	Severity  *int                   `json:"severity"`
	Message   string                 `json:"message,omitempty"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Metadata  map[string]string      `json:"metadata,omitempty"`
}

func IngestHandler(s store.PostgresStore, q chan queue.IngestionJob) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req ingestRequestInput
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
		}
		accepted := 0
		rejected := 0
		errs := []string{}
		acceptedSignals := make([]models.Signal, 0, len(req.Signals))
		for i := range req.Signals {
			sig := &req.Signals[i]
			signalErrs := []string{}

			if sig.EventType == "" || sig.Source == "" {
				signalErrs = append(signalErrs, "missing eventType or source")
			}
			if sig.EventType != "" && !allowedEventTypes[sig.EventType] {
				signalErrs = append(signalErrs, "invalid eventType")
			}
			if sig.Severity == nil {
				signalErrs = append(signalErrs, "missing severity")
			} else if *sig.Severity < 0 || *sig.Severity > 5 {
				signalErrs = append(signalErrs, "invalid severity")
			}

			var parsedTimestamp time.Time
			if sig.Timestamp != nil && *sig.Timestamp != "" {
				t, err := time.Parse(time.RFC3339, *sig.Timestamp)
				if err != nil {
					signalErrs = append(signalErrs, "invalid timestamp")
				} else {
					parsedTimestamp = t
				}
			}

			if len(signalErrs) > 0 {
				rejected++
				for _, msg := range signalErrs {
					errs = append(errs, fmt.Sprintf("signal %d: %s", i, msg))
				}
				continue
			}

			if sig.ID == "" {
				sig.ID = uuid.NewString()
			}

			validated := models.Signal{
				ID:        sig.ID,
				EventType: sig.EventType,
				Source:    sig.Source,
				Env:       sig.Env,
				Timestamp: parsedTimestamp,
				Severity:  *sig.Severity,
				Message:   sig.Message,
				Payload:   sig.Payload,
				Metadata:  sig.Metadata,
			}

			s.SaveSignal(validated)
			acceptedSignals = append(acceptedSignals, validated)
			accepted++
		}
		ingID := ""
		if len(acceptedSignals) > 0 {
			ingID = uuid.NewString()
			q <- queue.IngestionJob{IngestionID: ingID, Signals: acceptedSignals}
		}
		resp := models.IngestResponse{
			IngestionID:   ingID,
			AcceptedCount: accepted,
			RejectedCount: rejected,
			Errors:        errs,
		}
		return c.JSON(resp)
	}
}
