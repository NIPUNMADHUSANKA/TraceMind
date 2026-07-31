package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tracemind/internal/api"
	"tracemind/internal/models"
	"tracemind/internal/queue"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func setupIngestApp(t *testing.T) (*fiber.App, *queue.ReliableQueue) {
	t.Helper()

	app := fiber.New()
	s, cleanup := newTestPostgresStore(t)
	t.Cleanup(cleanup)
	q := queue.NewQueue()
	app.Post("/api/ingest", api.IngestHandler(s, q))
	return app, q
}

func postIngest(t *testing.T, app *fiber.App, body string) (*http.Response, models.IngestResponse) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	assert.NoError(t, err)

	var got models.IngestResponse
	if resp.StatusCode == http.StatusOK {
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	}
	return resp, got
}

func TestIngestValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		body          string
		expectedCode  int
		expectedOK    int
		expectedBad   int
		expectedError string
		extraErrors   []string
	}{
		{
			name:         "accepts valid signal",
			body:         `{"sourceContext":"local","signals":[{"eventType":"log","source":"svc","severity":5}]}`,
			expectedCode: http.StatusOK,
			expectedOK:   1,
			expectedBad:  0,
		},
		{
			name:         "accepts severity lower boundary",
			body:         `{"sourceContext":"local","signals":[{"eventType":"deployment","source":"svc","severity":0}]}`,
			expectedCode: http.StatusOK,
			expectedOK:   1,
			expectedBad:  0,
		},
		{
			name:         "accepts severity upper boundary",
			body:         `{"sourceContext":"local","signals":[{"eventType":"database","source":"svc","severity":5}]}`,
			expectedCode: http.StatusOK,
			expectedOK:   1,
			expectedBad:  0,
		},
		{
			name:          "rejects invalid eventType",
			body:          `{"sourceContext":"local","signals":[{"eventType":"oops","source":"svc","severity":4}]}`,
			expectedCode:  http.StatusOK,
			expectedOK:    0,
			expectedBad:   1,
			expectedError: "signal 0: invalid eventType",
		},
		{
			name:          "rejects severity below range",
			body:          `{"sourceContext":"local","signals":[{"eventType":"log","source":"svc","severity":-1}]}`,
			expectedCode:  http.StatusOK,
			expectedOK:    0,
			expectedBad:   1,
			expectedError: "signal 0: invalid severity",
		},
		{
			name:          "rejects severity above range",
			body:          `{"sourceContext":"local","signals":[{"eventType":"log","source":"svc","severity":6}]}`,
			expectedCode:  http.StatusOK,
			expectedOK:    0,
			expectedBad:   1,
			expectedError: "signal 0: invalid severity",
		},
		{
			name:          "rejects missing severity",
			body:          `{"sourceContext":"local","signals":[{"eventType":"log","source":"svc"}]}`,
			expectedCode:  http.StatusOK,
			expectedOK:    0,
			expectedBad:   1,
			expectedError: "signal 0: missing severity",
		},
		{
			name:         "accepts missing timestamp",
			body:         `{"sourceContext":"local","signals":[{"eventType":"health","source":"svc","severity":3}]}`,
			expectedCode: http.StatusOK,
			expectedOK:   1,
			expectedBad:  0,
		},
		{
			name:          "rejects invalid timestamp",
			body:          `{"sourceContext":"local","signals":[{"eventType":"log","source":"svc","severity":2,"timestamp":"not-a-time"}]}`,
			expectedCode:  http.StatusOK,
			expectedOK:    0,
			expectedBad:   1,
			expectedError: "signal 0: invalid timestamp",
		},
		{
			name:          "rejects missing source",
			body:          `{"sourceContext":"local","signals":[{"eventType":"log","severity":2}]}`,
			expectedCode:  http.StatusOK,
			expectedOK:    0,
			expectedBad:   1,
			expectedError: "signal 0: missing source",
		},
		{
			name:         "rejects empty signals batch",
			body:         `{"sourceContext":"local","signals":[]}`,
			expectedCode: http.StatusBadRequest,
			expectedOK:   0,
			expectedBad:  0,
		},
		{
			name:          "reports multiple errors for one signal",
			body:          `{"sourceContext":"local","signals":[{"eventType":"bad","source":"svc","severity":9,"timestamp":"bad-time"}]}`,
			expectedCode:  http.StatusOK,
			expectedOK:    0,
			expectedBad:   1,
			expectedError: "signal 0: invalid eventType",
			extraErrors: []string{
				"signal 0: invalid severity",
				"signal 0: invalid timestamp",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app, _ := setupIngestApp(t)
			resp, got := postIngest(t, app, tc.body)

			assert.Equal(t, tc.expectedCode, resp.StatusCode)
			assert.Equal(t, tc.expectedOK, got.AcceptedCount)
			assert.Equal(t, tc.expectedBad, got.RejectedCount)
			if tc.expectedError != "" {
				assert.Contains(t, strings.Join(got.Errors, " "), tc.expectedError)
			}
			for _, wantErr := range tc.extraErrors {
				assert.Contains(t, strings.Join(got.Errors, " "), wantErr)
			}
		})
	}
}

func TestIngestValidation_QueuesOnlyAcceptedSignals(t *testing.T) {
	t.Parallel()

	app, q := setupIngestApp(t)
	body := `{"sourceContext":"local","signals":[{"eventType":"log","source":"svc-a","severity":5},{"eventType":"unknown","source":"svc-b","severity":5},{"eventType":"queue","source":"svc-c","severity":2,"timestamp":"bad-time"}]}`

	resp, got := postIngest(t, app, body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 1, got.AcceptedCount)
	assert.Equal(t, 2, got.RejectedCount)

	delivery, err := q.Dequeue(context.Background())
	assert.NoError(t, err)
	assert.Len(t, delivery.Job.Signals, 1)
	assert.Equal(t, "svc-a", delivery.Job.Signals[0].Source)
	assert.Equal(t, "log", delivery.Job.Signals[0].EventType)
}

func TestIngestValidation_AllRejectedHasNoIngestionID(t *testing.T) {
	t.Parallel()

	app, q := setupIngestApp(t)
	body := `{"sourceContext":"local","signals":[{"eventType":"unknown","source":"svc-a","severity":7}]}`

	resp, got := postIngest(t, app, body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 0, got.AcceptedCount)
	assert.Equal(t, 1, got.RejectedCount)
	assert.Equal(t, "", got.IngestionID)

	_, err := q.Dequeue(context.Background())
	assert.Error(t, err)
	assert.True(t, errors.Is(err, queue.ErrQueueEmpty))
}
