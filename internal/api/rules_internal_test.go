package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestParseAnalysisRuleRequest_DefaultsPriorityAndVersion(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Post("/", func(c *fiber.Ctx) error {
		rule, err := parseAnalysisRuleRequest(c)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"priority": rule.Priority, "version": rule.Version})
	})

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{
		"name":"Queue Backlog",
		"hypothesisTemplate":"queue backlog is hurting processing latency",
		"matchType":"single"
	}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, float64(100), body["priority"])
	assert.Equal(t, float64(1), body["version"])
}

func TestParseAnalysisRuleRequest_RejectsVersionZeroWithClearMessage(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Post("/", func(c *fiber.Ctx) error {
		rule, err := parseAnalysisRuleRequest(c)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"priority": rule.Priority, "version": rule.Version})
	})

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{
		"name":"Queue Backlog",
		"hypothesisTemplate":"queue backlog is hurting processing latency",
		"matchType":"single",
		"version":0
	}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var body map[string]interface{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Contains(t, strings.ToLower(body["error"].(string)), "version must be at least 1")
}
