package e2e_test

import (
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

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

func TestQueueAnalysisArchiveE2E(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is required for e2e tests with PostgresStore")
	}

	ps, err := store.NewPostgresStore(dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, ps.Close())
	})
	st := *ps

	q := queue.NewReliableQueue(queue.QueueConfig{MaxAttempts: 2, VisibilityTimeout: 20 * time.Millisecond})
	stopCh := make(chan struct{})
	worker.StartWorker(q, st, stopCh)
	t.Cleanup(func() {
		close(stopCh)
	})

	app := fiber.New()
	app.Post("/api/ingest", api.IngestHandler(st, q))
	app.Get("/api/incidents", api.IncidentsHandler(st))
	app.Get("/api/health/ingestion", api.HealthHandler(q, st))

	body := `{"sourceContext":"e2e","signals":[{"id":"e2e-db-1","eventType":"database","source":"checkout","environment":"prod","severity":5,"message":"too many connections"},{"id":"e2e-health-1","eventType":"health","source":"checkout","environment":"prod","severity":4,"message":"service timeout"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Eventually(t, func() bool {
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

		ingestion, ok := healthPayload["ingestion"].(map[string]any)
		if !ok {
			return false
		}
		if _, ok := ingestion["deadLetterCount"]; !ok {
			return false
		}
		if _, ok := ingestion["retryCount"]; !ok {
			return false
		}
		return true
	}, 5*time.Second, 100*time.Millisecond)
}
