package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"tracemind/internal/api"
	"tracemind/internal/queue"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

type staticQueueStats struct {
	health *queue.QueueHealth
	err    error
}

func (s staticQueueStats) Health(context.Context) (*queue.QueueHealth, error) {
	return s.health, s.err
}

func TestHealthHandler_IncludesQueueLifecycleMetrics(t *testing.T) {
	t.Parallel()

	expected := &queue.QueueHealth{
		Available: "4",
		InFlight:  "2",
		Delayed:   "1",
	}

	app := fiber.New()
	app.Get("/api/health/ingestion", api.HealthHandler(staticQueueStats{health: expected}))

	req := httptest.NewRequest(http.MethodGet, "/api/health/ingestion", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var payload map[string]interface{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	assert.Equal(t, "healthy", payload["status"])
	assert.EqualValues(t, expected.Available, payload["available"])
	assert.EqualValues(t, expected.InFlight, payload["inFlight"])
	assert.EqualValues(t, expected.Delayed, payload["delayed"])
}
