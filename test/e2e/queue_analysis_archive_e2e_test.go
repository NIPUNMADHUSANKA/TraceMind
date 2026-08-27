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

type queueArchiveE2EQueueAdapter struct {
	*queue.ReliableQueue
}

func newQueueArchiveE2EQueueAdapter(cfg queue.QueueConfig) *queueArchiveE2EQueueAdapter {
	return &queueArchiveE2EQueueAdapter{ReliableQueue: queue.NewReliableQueue(cfg)}
}

func (q *queueArchiveE2EQueueAdapter) Enqueue(ctx context.Context, job queue.IngestionJob) error {
	_ = ctx
	return q.ReliableQueue.Enqueue(job)
}

func (q *queueArchiveE2EQueueAdapter) Dequeue(ctx context.Context) (*types.Message, error) {
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

func (q *queueArchiveE2EQueueAdapter) Ack(receipt string, ctx context.Context) error {
	_ = ctx
	return q.ReliableQueue.Ack(receipt)
}

func (q *queueArchiveE2EQueueAdapter) Nack(receipt string, ctx context.Context) error {
	_ = ctx
	return q.ReliableQueue.Nack(receipt, "")
}

func (q *queueArchiveE2EQueueAdapter) Health(context.Context) (*queue.QueueHealth, error) {
	return &queue.QueueHealth{Available: "0", InFlight: "0", Delayed: "0"}, nil
}

func TestQueueAnalysisArchiveE2E(t *testing.T) {
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

	q := newQueueArchiveE2EQueueAdapter(queue.QueueConfig{MaxAttempts: 2, VisibilityTimeout: 20 * time.Millisecond})
	stopCh := make(chan struct{})
	worker.StartWorker(q, st, stopCh)
	t.Cleanup(func() {
		close(stopCh)
	})

	app := fiber.New()
	app.Post("/api/ingest", api.IngestHandler(st, q))
	app.Get("/api/incidents", api.IncidentsHandler(st))
	app.Get("/api/health/ingestion", api.HealthHandler(q))

	body := `{"sourceContext":"e2e","signals":[{"id":"e2e-db-1","eventType":"database","source":"checkout","environment":"prod","severity":5,"message":"too many connections"},{"id":"e2e-health-1","eventType":"health","source":"checkout","environment":"prod","severity":4,"message":"service timeout"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	assert.Eventually(t, func() bool {
		incReq := httptest.NewRequest(http.MethodGet, "/api/incidents", nil)
		incResp, incErr := app.Test(incReq)
		if incErr != nil || incResp.StatusCode != http.StatusOK {
			return false
		}
		defer incResp.Body.Close()

		var incPayload struct {
			Incidents []models.Incident `json:"incidents"`
		}
		if decodeErr := json.NewDecoder(incResp.Body).Decode(&incPayload); decodeErr != nil {
			return false
		}

		matched := false
		for _, inc := range incPayload.Incidents {
			if !contains(inc.SignalIDs, "e2e-db-1") {
				continue
			}
			if inc.AnalysisSummary == "" || len(inc.Recommendations) == 0 {
				return false
			}
			matched = true
			break
		}
		if !matched {
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
	}, 5*time.Second, 100*time.Millisecond)
}
