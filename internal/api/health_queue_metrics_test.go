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
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload struct {
		Ingestion map[string]any `json:"ingestion"`
		Incidents int            `json:"incidents"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.NotNil(t, payload.Ingestion)

	require.EqualValues(t, expected.Depth, payload.Ingestion["queueDepth"])
	require.EqualValues(t, expected.RetryCount, payload.Ingestion["retryCount"])
	require.EqualValues(t, expected.DeadLetterCount, payload.Ingestion["deadLetterCount"])

	tsRaw, ok := payload.Ingestion["lastProcessedTimestamp"].(string)
	require.True(t, ok)
	parsedTs, parseErr := time.Parse(time.RFC3339, tsRaw)
	require.NoError(t, parseErr)
	require.WithinDuration(t, expected.LastProcessedTimestamp, parsedTs, time.Second)
}
