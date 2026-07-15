package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tracemind/internal/api"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

func setupPayloadFilterApp(t *testing.T) (*fiber.App, func() []string) {
	t.Helper()

	app := fiber.New()
	s, cleanup := newTestPostgresStore(t)
	t.Cleanup(cleanup)

	app.Put("/api/payload-filters/:environment", api.PayloadFilter(s))
	app.Delete("/api/payload-filters/:environment", api.DeletePayloadFilter(s))

	readBack := func() []string {
		allowList, err := s.GetPayloadFilterConfig("staging")
		require.NoError(t, err)
		return allowList
	}

	return app, readBack
}

func TestPayloadFilter_InvalidJSON(t *testing.T) {
	t.Parallel()

	app, _ := setupPayloadFilterApp(t)

	req := httptest.NewRequest(http.MethodPut, "/api/payload-filters/staging", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, "request body must be valid JSON", body["error"])
}

func TestPayloadFilter_RejectsEmptyPayloads(t *testing.T) {
	t.Parallel()

	app, _ := setupPayloadFilterApp(t)

	req := httptest.NewRequest(http.MethodPut, "/api/payload-filters/staging", strings.NewReader(`{"payloads":["", "   "]}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, "payloads must contain at least one key", body["error"])
}

func TestPayloadFilter_UpdatesAllowListAndReturnsMessage(t *testing.T) {
	t.Parallel()

	app, readBack := setupPayloadFilterApp(t)

	req := httptest.NewRequest(http.MethodPut, "/api/payload-filters/staging", strings.NewReader(`{"payloads":["requestId"," traceId ","requestId"]}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, "success", body["status"])
	require.Equal(t, "payload allow-list updated", body["message"])
	require.Equal(t, "staging", body["environment"])
	require.Equal(t, float64(2), body["count"])

	allowList := readBack()
	require.ElementsMatch(t, []string{"requestId", "traceId"}, allowList)
}

func TestDeletePayloadFilter_InvalidJSON(t *testing.T) {
	t.Parallel()

	app, _ := setupPayloadFilterApp(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/payload-filters/staging", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, "request body must be valid JSON", body["error"])
}

func TestDeletePayloadFilter_RejectsEmptyPayloads(t *testing.T) {
	t.Parallel()

	app, _ := setupPayloadFilterApp(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/payload-filters/staging", strings.NewReader(`{"payloads":["", "   "]}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, "payloads must contain at least one key", body["error"])
}

func TestDeletePayloadFilter_RemovesPayloadsAndReturnsMessage(t *testing.T) {
	t.Parallel()

	app, readBack := setupPayloadFilterApp(t)

	seedReq := httptest.NewRequest(http.MethodPut, "/api/payload-filters/staging", strings.NewReader(`{"payloads":["requestId","traceId","sessionId"]}`))
	seedReq.Header.Set("Content-Type", "application/json")
	seedResp, err := app.Test(seedReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, seedResp.StatusCode)

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/payload-filters/staging", strings.NewReader(`{"payloads":[" traceId ", "traceId", "sessionId"]}`))
	deleteReq.Header.Set("Content-Type", "application/json")

	deleteResp, err := app.Test(deleteReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, deleteResp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(deleteResp.Body).Decode(&body))
	require.Equal(t, "success", body["status"])
	require.Equal(t, "payload allow-list updated", body["message"])
	require.Equal(t, "staging", body["environment"])
	require.Equal(t, float64(2), body["count"])

	allowList := readBack()
	require.ElementsMatch(t, []string{"requestId"}, allowList)
}
