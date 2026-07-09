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

func TestIngestCreatesIncidentAndListsViaAPI(t *testing.T) {
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

	q := queue.NewQueue(10)
	stopCh := make(chan struct{})
	worker.StartWorker(q, st, stopCh)
	t.Cleanup(func() {
		close(stopCh)
	})

	app := fiber.New()
	app.Post("/api/ingest", api.IngestHandler(st, q))
	app.Get("/api/incidents", api.IncidentsHandler(st))

	ingestBody := `{"sourceContext":"e2e","signals":[{"id":"e2e-signal-high","eventType":"log","source":"e2e-service","environment":"prod","severity":5,"message":"critical failure"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/ingest", strings.NewReader(ingestBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var ingestResp models.IngestResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&ingestResp))
	require.Equal(t, 1, ingestResp.AcceptedCount)
	require.NotEmpty(t, ingestResp.IngestionID)

	require.Eventually(t, func() bool {
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
		for _, inc := range payload.Incidents {
			if inc.Severity >= 4 && contains(inc.SignalIDs, "e2e-signal-high") && contains(inc.ImpactedServices, "e2e-service") {
				return true
			}
		}
		return false
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
