package e2e_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
	"tracemind/internal/api"
	"tracemind/internal/models"
	"tracemind/internal/queue"
	"tracemind/internal/store"
	"tracemind/internal/worker"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

type e2eQueueAdapter struct {
	*queue.ReliableQueue
}

func newE2EQueueAdapter(cfg queue.QueueConfig) *e2eQueueAdapter {
	return &e2eQueueAdapter{ReliableQueue: queue.NewReliableQueue(cfg)}
}

func (q *e2eQueueAdapter) Enqueue(ctx context.Context, job queue.IngestionJob) error {
	_ = ctx
	return q.ReliableQueue.Enqueue(job)
}

func (q *e2eQueueAdapter) Dequeue(ctx context.Context) (*types.Message, error) {
	delivery, err := q.ReliableQueue.Dequeue(ctx)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(delivery.Job)
	if err != nil {
		return nil, err
	}
	return &types.Message{
		Body:          aws.String(string(body)),
		ReceiptHandle: aws.String(delivery.Receipt),
		MessageId:     aws.String(delivery.Receipt),
	}, nil
}

func (q *e2eQueueAdapter) Ack(receipt string, ctx context.Context) error {
	_ = ctx
	return q.ReliableQueue.Ack(receipt)
}

func (q *e2eQueueAdapter) Nack(receipt string, ctx context.Context) error {
	_ = ctx
	return q.ReliableQueue.Nack(receipt, "")
}

func (q *e2eQueueAdapter) Health(context.Context) (*queue.QueueHealth, error) {
	return &queue.QueueHealth{Available: "0", InFlight: "0", Delayed: "0"}, nil
}

func TestIngestCreatesIncidentAndListsViaAPI(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is assertd for e2e tests with PostgresStore")
	}

	ps, err := store.NewPostgresStore(dsn)
	assert.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, ps.Close())
	})
	st := *ps

	q := newE2EQueueAdapter(queue.QueueConfig{MaxAttempts: 3})
	stopCh := make(chan struct{})
	worker.StartWorker(q, st, stopCh)
	t.Cleanup(func() {
		close(stopCh)
	})

	app := fiber.New()
	app.Post("/api/ingest", api.IngestHandler(st, q))
	app.Get("/api/incidents", api.IncidentsHandler(st))
	app.Get("/api/health/ingestion", api.HealthHandler(q))

	ingestBody := `{"sourceContext":"e2e","signals":[{"id":"e2e-signal-high","eventType":"log","source":"e2e-service","environment":"prod","severity":5,"message":"critical failure"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/ingest", strings.NewReader(ingestBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var ingestResp models.IngestResponse
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&ingestResp))
	assert.Equal(t, 1, ingestResp.AcceptedCount)
	assert.NotEmpty(t, ingestResp.IngestionID)

	assert.Eventually(t, func() bool {
		incReq := httptest.NewRequest(http.MethodGet, "/api/incidents", nil)
		incResp, testErr := app.Test(incReq)
		if testErr != nil || incResp.StatusCode != http.StatusOK {
			return false
		}
		defer incResp.Body.Close()

		var payload struct {
			Incidents []models.Incident `json:"incidents"`
		}
		if decodeErr := json.NewDecoder(incResp.Body).Decode(&payload); decodeErr != nil {
			return false
		}
		matchedIncident := false
		for _, inc := range payload.Incidents {
			if inc.Severity >= 4 && contains(inc.SignalIDs, "e2e-signal-high") && contains(inc.ImpactedServices, "e2e-service") {
				if inc.AnalysisSummary == "" || len(inc.Recommendations) == 0 {
					return false
				}
				matchedIncident = true
				break
			}
		}
		if !matchedIncident {
			return false
		}

		healthReq := httptest.NewRequest(http.MethodGet, "/api/health/ingestion", nil)
		healthResp, healthErr := app.Test(healthReq)
		if healthErr != nil || healthResp.StatusCode != http.StatusOK {
			return false
		}
		defer healthResp.Body.Close()

		var healthPayload map[string]any
		if decodeErr := json.NewDecoder(healthResp.Body).Decode(&healthPayload); decodeErr != nil {
			return false
		}

		if _, ok := healthPayload["status"]; !ok {
			return false
		}
		if _, ok := healthPayload["available"]; !ok {
			return false
		}
		if _, ok := healthPayload["inFlight"]; !ok {
			return false
		}
		if _, ok := healthPayload["delayed"]; !ok {
			return false
		}
		return true
	}, 3*time.Second, 100*time.Millisecond)
}

func contains(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
