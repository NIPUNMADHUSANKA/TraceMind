package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"tracemind/internal/api"
	"tracemind/internal/queue"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

type staticQueueStats struct {
	stats queue.QueueStats
}

func (s staticQueueStats) Stats() queue.QueueStats {
	return s.stats
}

func TestHealthHandler_IncludesQueueLifecycleMetrics(t *testing.T) {
	t.Parallel()

	st, cleanup := newTestPostgresStore(t)
	t.Cleanup(cleanup)

	expected := queue.QueueStats{
		Depth:                  4,
		RetryCount:             2,
		DeadLetterCount:        1,
		LastProcessedTimestamp: time.Now().UTC().Truncate(time.Second),
	}

	app := fiber.New()
	app.Get("/api/health/ingestion", api.HealthHandler(staticQueueStats{stats: expected}, st))

	req := httptest.NewRequest(http.MethodGet, "/api/health/ingestion", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var payload struct {
		Ingestion map[string]interface{} `json:"ingestion"`
		Incidents int                    `json:"incidents"`
	}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	assert.NotNil(t, payload.Ingestion)

	assert.EqualValues(t, expected.Depth, payload.Ingestion["queueDepth"])
	assert.EqualValues(t, expected.RetryCount, payload.Ingestion["retryCount"])
	assert.EqualValues(t, expected.DeadLetterCount, payload.Ingestion["deadLetterCount"])
}
