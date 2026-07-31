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

func setupRulesApp(t *testing.T) *fiber.App {
	t.Helper()

	app := fiber.New()
	s, cleanup := newTestPostgresStore(t)
	t.Cleanup(cleanup)

	app.Post("/api/analysis-rules", api.CreateAnalysisRuleHandler(s))
	app.Put("/api/analysis-rules/:id", api.UpdateAnalysisRuleHandler(s))
	app.Delete("/api/analysis-rules/:id", api.DeleteAnalysisRuleHandler(s))

	app.Post("/api/analysis-rule-patterns", api.CreateAnalysisRulePatternHandler(s))
	app.Put("/api/analysis-rule-patterns/:id", api.UpdateAnalysisRulePatternHandler(s))
	app.Delete("/api/analysis-rule-patterns/:id", api.DeleteAnalysisRulePatternHandler(s))

	return app
}

func sendJSONRequest(t *testing.T, app *fiber.App, method string, path string, body string) (*http.Response, map[string]interface{}) {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	assert.NoError(t, err)

	defer resp.Body.Close()
	parsed := map[string]interface{}{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&parsed))
	return resp, parsed
}

func createRuleAndGetID(t *testing.T, app *fiber.App) string {
	t.Helper()

	resp, body := sendJSONRequest(t, app, http.MethodPost, "/api/analysis-rules", `{"name":"DB Failure","hypothesisTemplate":"database connection degraded","recommendations":["inspect pool limits"]}`)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	idVal, ok := body["id"].(string)
	assert.True(t, ok)
	assert.NotEmpty(t, idVal)
	return idVal
}

func TestAnalysisRuleCRUDFlow(t *testing.T) {
	t.Parallel()

	app := setupRulesApp(t)

	createResp, createBody := sendJSONRequest(t, app, http.MethodPost, "/api/analysis-rules", `{
		"name":"Queue Backlog",
		"description":"detects queue delays",
		"category":"queue",
		"severity":"high",
		"confidence":"medium",
		"priority":80,
		"enabled":true,
		"matchType":"single",
		"hypothesisTemplate":"queue backlog is hurting processing latency",
		"recommendations":["scale consumers"],
		"version":2
	}`)
	assert.Equal(t, http.StatusCreated, createResp.StatusCode)

	ruleID, ok := createBody["id"].(string)
	assert.True(t, ok)
	assert.NotEmpty(t, ruleID)

	updateResp, _ := sendJSONRequest(t, app, http.MethodPut, "/api/analysis-rules/"+ruleID, `{
		"name":"Queue Backlog Updated",
		"description":"updated",
		"category":"queue",
		"severity":"critical",
		"confidence":"high",
		"priority":70,
		"enabled":false,
		"matchType":"single",
		"hypothesisTemplate":"queue backlog now critical",
		"recommendations":["scale workers","inspect retries"],
		"version":3
	}`)
	assert.Equal(t, http.StatusOK, updateResp.StatusCode)

	deleteResp, _ := sendJSONRequest(t, app, http.MethodDelete, "/api/analysis-rules/"+ruleID, `{}`)
	assert.Equal(t, http.StatusOK, deleteResp.StatusCode)

	notFoundResp, _ := sendJSONRequest(t, app, http.MethodDelete, "/api/analysis-rules/"+ruleID, `{}`)
	assert.Equal(t, http.StatusNotFound, notFoundResp.StatusCode)
}

func TestAnalysisRulePatternCRUDFlow(t *testing.T) {
	t.Parallel()

	app := setupRulesApp(t)
	ruleID := createRuleAndGetID(t, app)

	createResp, createBody := sendJSONRequest(t, app, http.MethodPost, "/api/analysis-rule-patterns", `{
		"ruleId":"`+ruleID+`",
		"eventType":"database",
		"source":"checkout",
		"environment":"prod",
		"severityMin":4,
		"messageMatchType":"contains",
		"messagePattern":"connection",
		"payloadConditions":[{"path":"db.host","operator":"exists"}],
		"variableMappings":{"service":"checkout"}
	}`)
	assert.Equal(t, http.StatusCreated, createResp.StatusCode)

	patternID, ok := createBody["id"].(string)
	assert.True(t, ok)
	assert.NotEmpty(t, patternID)

	updateResp, _ := sendJSONRequest(t, app, http.MethodPut, "/api/analysis-rule-patterns/"+patternID, `{
		"ruleId":"`+ruleID+`",
		"eventType":"database",
		"source":"checkout",
		"environment":"staging",
		"severityMin":3,
		"messageMatchType":"contains",
		"messagePattern":"timeout",
		"payloadConditions":[{"path":"db.pool","operator":"gt","value":90}],
		"variableMappings":{"service":"checkout","region":"us-east-1"}
	}`)
	assert.Equal(t, http.StatusOK, updateResp.StatusCode)

	deleteResp, _ := sendJSONRequest(t, app, http.MethodDelete, "/api/analysis-rule-patterns/"+patternID, `{}`)
	assert.Equal(t, http.StatusOK, deleteResp.StatusCode)

	notFoundResp, _ := sendJSONRequest(t, app, http.MethodDelete, "/api/analysis-rule-patterns/"+patternID, `{}`)
	assert.Equal(t, http.StatusNotFound, notFoundResp.StatusCode)
}
