package api

import (
	"bufio"
	"context"
	"fmt"
	"time"
	"tracemind/internal/models"
	"tracemind/internal/queue"
	"tracemind/internal/store"
	"tracemind/internal/util"

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

type ingestQueue interface {
	Enqueue(ctx context.Context, job queue.IngestionJob) error
}

func IngestHandler(s store.PostgresStore, q ingestQueue) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req ingestRequestInput
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		} else if len(req.Signals) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "signals is required and must contain at least one item"})
		}
		accepted := 0
		duplicates := 0
		rejected := 0
		errs := []string{}
		acceptedSignals := make([]models.Signal, 0, len(req.Signals))
		for i := range req.Signals {
			sig := &req.Signals[i]
			signalErrs := []string{}

			if sig.EventType == "" {
				signalErrs = append(signalErrs, "missing eventType")
			}
			if sig.Source == "" {
				signalErrs = append(signalErrs, "missing source")
			}
			if sig.EventType != "" && !allowedEventTypes[sig.EventType] {
				signalErrs = append(signalErrs, "invalid eventType")
			}
			if sig.Severity == nil {
				signalErrs = append(signalErrs, "missing severity")
			} else if *sig.Severity < 0 || *sig.Severity > 5 {
				signalErrs = append(signalErrs, "invalid severity")
			}
			if _, err := util.FormatEnvironment(sig.Env); err != nil {
				signalErrs = append(signalErrs, err.Error())
			}

			var parsedTimestamp time.Time
			if sig.Timestamp != nil && *sig.Timestamp != "" {
				t, err := time.Parse(time.RFC3339, *sig.Timestamp)
				if err != nil {
					signalErrs = append(signalErrs, "invalid timestamp")
				} else {
					parsedTimestamp = t
				}
			} else {
				parsedTimestamp = time.Now().UTC()
			}

			// Message is optional; it may be empty depending on the signal type.

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

			environment, _ := util.FormatEnvironment(sig.Env)

			validated := models.Signal{
				ID:        sig.ID,
				EventType: sig.EventType,
				Source:    sig.Source,
				Env:       environment,
				Timestamp: parsedTimestamp,
				Severity:  *sig.Severity,
				Message:   sig.Message,
				Payload:   sig.Payload,
				Metadata:  sig.Metadata,
			}

			created, err := s.CreateSignal(validated)
			if err != nil {
				rejected++
				errs = append(errs, fmt.Sprintf("signal %d: failed to persist signal", i))
				continue
			}

			if !created {
				duplicates++
				continue
			}

			acceptedSignals = append(acceptedSignals, validated)
			accepted++
		}
		ingID := ""
		if len(acceptedSignals) > 0 {
			ingID = uuid.NewString()
			if err := s.CreateIngestionStatus(ingID, "pending"); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
			job := queue.IngestionJob{IngestionID: ingID, Signals: acceptedSignals}
			if err := q.Enqueue(c.UserContext(), job); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
		}
		resp := models.IngestResponse{
			IngestionID:    ingID,
			AcceptedCount:  accepted,
			DuplicateCount: duplicates,
			RejectedCount:  rejected,
			Errors:         errs,
		}
		return c.Status(fiber.StatusOK).JSON(resp)
	}
}

func HandleSSE(s store.PostgresStore, broker *util.Broker) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ingestionID := c.Params("id")
		if ingestionID == "" {
			return c.Status(fiber.StatusBadRequest).SendString("missing ingestion ID")
		}

		c.Set(fiber.HeaderContentType, "text/event-stream")
		c.Set(fiber.HeaderCacheControl, "no-cache")
		c.Set(fiber.HeaderConnection, "keep-alive")
		c.Set("Access-Control-Allow-Origin", "*")

		ch := broker.Subscribe(ingestionID)

		status, found, err := s.GetIngestionStatus(ingestionID)

		if err != nil {
			broker.Unsubscribe(ingestionID, ch)
			return c.Status(fiber.StatusInternalServerError).
				SendString("failed to get ingestion status")
		}

		if !found {
			broker.Unsubscribe(ingestionID, ch)
			return c.Status(fiber.StatusNotFound).
				SendString("ingestion not found")
		}

		c.Context().SetBodyStreamWriter(func(writer *bufio.Writer) {
			defer broker.Unsubscribe(ingestionID, ch)

			if _, err := fmt.Fprintf(writer, "id: %s\nstatus: %s\n\n", ingestionID, status.Status); err != nil {
				return
			}
			if err := writer.Flush(); err != nil {
				return
			}
			if status.Status == "completed" || status.Status == "failed" {
				return
			}

			for {
				event, ok := <-ch

				if !ok {
					return
				}

				if _, err := fmt.Fprint(
					writer,
					event.Format(),
				); err != nil {
					return
				}

				if err := writer.Flush(); err != nil {
					return
				}

				if event.Status == "completed" || event.Status == "failed" {
					return
				}
			}
		})

		return nil
	}
}
