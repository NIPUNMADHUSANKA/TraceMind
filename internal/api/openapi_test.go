package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"tracemind/internal/api"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestOpenAPISpecHandler_ReturnsJSON(t *testing.T) {
	app := fiber.New()
	app.Get("/api/openapi.json", api.OpenAPISpecHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/openapi.json", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, strings.ToLower(resp.Header.Get("Content-Type")), "application/json")

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	bodyStr := string(body)
	assert.Contains(t, bodyStr, "\"openapi\": \"3.0.3\"")
	assert.Contains(t, bodyStr, "\"/api/ingest\"")
	assert.Contains(t, bodyStr, "\"/api/payload-filters/{environment}\"")
}

func TestSwaggerUIHandler_ReturnsHTML(t *testing.T) {
	app := fiber.New()
	app.Get("/docs", api.SwaggerUIHandler())

	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, strings.ToLower(resp.Header.Get("Content-Type")), "text/html")

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	bodyStr := string(body)
	assert.Contains(t, bodyStr, "TraceMind OpenAPI / Swagger")
	assert.Contains(t, bodyStr, "url: '/api/openapi.json'")
	assert.Contains(t, bodyStr, "operationsSorter: 'alpha'")
}

func TestOpenAPIOnboardingDocumentation(t *testing.T) {
	app := fiber.New()
	app.Get("/api/openapi.json", api.OpenAPISpecHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/openapi.json", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	bodyStr := string(body)

	assert.Contains(t, bodyStr, "Typical workflow")
	assert.Contains(t, bodyStr, "Optional setup")
	assert.Contains(t, bodyStr, "keep the returned ingestionId")
	assert.Contains(t, bodyStr, "Server-sent event stream")
	assert.Less(t, strings.Index(bodyStr, "Typical workflow"), strings.Index(bodyStr, `"paths": {`))

	var spec struct {
		Paths map[string]map[string]struct {
			Description string `json:"description"`
		} `json:"paths"`
	}
	assert.NoError(t, json.Unmarshal(body, &spec))
	for path, methods := range spec.Paths {
		for method, operation := range methods {
			assert.NotEmpty(t, operation.Description, "%s %s is missing an operation description", method, path)
		}
	}
}
