package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tracemind/internal/api"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func setupPayloadFilterApp(t *testing.T) (*fiber.App, func() []string) {
	t.Helper()

	app := fiber.New()
	s, cleanup := newTestPostgresStore(t)
	t.Cleanup(cleanup)

	app.Post("/api/payload-filters/:environment", api.PayloadFilter(s))
	app.Delete("/api/payload-filters/:environment", api.DeletePayloadFilter(s))

	readBack := func() []string {
		allowList, err := s.GetPayloadFilterConfig("staging")
		assert.NoError(t, err)
		return allowList
	}

	return app, readBack
}

func TestPayloadFilter_InvalidJSON(t *testing.T) {
	t.Parallel()

	app, _ := setupPayloadFilterApp(t)

	req := httptest.NewRequest(http.MethodPost, "/api/payload-filters/staging", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var body map[string]string
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "request body must be valid JSON", body["error"])
}

func TestPayloadFilter_RejectsEmptyPayloads(t *testing.T) {
	t.Parallel()

	app, _ := setupPayloadFilterApp(t)

	req := httptest.NewRequest(http.MethodPost, "/api/payload-filters/staging", strings.NewReader(`{"payloads":["", "   "]}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var body map[string]string
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "payloads must contain at least one key", body["error"])
}

func TestPayloadFilter_UpdatesAllowListAndReturnsMessage(t *testing.T) {
	t.Parallel()

	app, readBack := setupPayloadFilterApp(t)

	req := httptest.NewRequest(http.MethodPost, "/api/payload-filters/staging", strings.NewReader(`{"payloads":["requestId"," traceId ","requestId"]}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "success", body["status"])
	assert.Equal(t, "payload allow-list updated", body["message"])
	assert.Equal(t, "staging", body["environment"])
	assert.Equal(t, float64(2), body["count"])

	allowList := readBack()
	assert.ElementsMatch(t, []string{"requestId", "traceId"}, allowList)
}

func TestPayloadFilter_DuplicateRequestIsIdempotent(t *testing.T) {
	t.Parallel()

	app, _ := setupPayloadFilterApp(t)

	firstReq := httptest.NewRequest(http.MethodPost, "/api/payload-filters/prod", strings.NewReader(`{"payloads":["requestId","traceId"]}`))
	firstReq.Header.Set("Content-Type", "application/json")

	firstResp, err := app.Test(firstReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, firstResp.StatusCode)

	secondReq := httptest.NewRequest(http.MethodPost, "/api/payload-filters/prod", strings.NewReader(`{"payloads":["traceId","requestId"]}`))
	secondReq.Header.Set("Content-Type", "application/json")

	secondResp, err := app.Test(secondReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, secondResp.StatusCode)

	var body map[string]interface{}
	assert.NoError(t, json.NewDecoder(secondResp.Body).Decode(&body))
	assert.Equal(t, "success", body["status"])
	assert.Equal(t, "payload allow-list updated", body["message"])
	assert.Equal(t, "prod", body["environment"])
	assert.Equal(t, float64(2), body["count"])
	assert.NotContains(t, body, "error")
}

func TestDeletePayloadFilter_InvalidJSON(t *testing.T) {
	t.Parallel()

	app, _ := setupPayloadFilterApp(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/payload-filters/staging", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var body map[string]string
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "request body must be valid JSON", body["error"])
}

func TestDeletePayloadFilter_RejectsEmptyPayloads(t *testing.T) {
	t.Parallel()

	app, _ := setupPayloadFilterApp(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/payload-filters/staging", strings.NewReader(`{"payloads":["", "   "]}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var body map[string]string
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "payloads must contain at least one key", body["error"])
}

func TestDeletePayloadFilter_RemovesPayloadsAndReturnsMessage(t *testing.T) {
	t.Parallel()

	app, readBack := setupPayloadFilterApp(t)

	seedReq := httptest.NewRequest(http.MethodPost, "/api/payload-filters/staging", strings.NewReader(`{"payloads":["requestId","traceId","sessionId"]}`))
	seedReq.Header.Set("Content-Type", "application/json")
	seedResp, err := app.Test(seedReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, seedResp.StatusCode)

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/payload-filters/staging", strings.NewReader(`{"payloads":[" traceId ", "traceId", "sessionId"]}`))
	deleteReq.Header.Set("Content-Type", "application/json")

	deleteResp, err := app.Test(deleteReq)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, deleteResp.StatusCode)

	var body map[string]interface{}
	assert.NoError(t, json.NewDecoder(deleteResp.Body).Decode(&body))
	assert.Equal(t, "success", body["status"])
	assert.Equal(t, "payload allow-list updated", body["message"])
	assert.Equal(t, "staging", body["environment"])
	assert.Equal(t, float64(2), body["count"])

	allowList := readBack()
	assert.ElementsMatch(t, []string{"requestId"}, allowList)
}
