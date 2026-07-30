package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tracemind/internal/api"
	"tracemind/internal/queue"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func setupApp(t *testing.T) *fiber.App {
	t.Helper()

	app := fiber.New()
	s, cleanup := newTestPostgresStore(t)
	t.Cleanup(cleanup)
	q := queue.NewQueue()
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
	app.Post("/api/ingest", api.IngestHandler(s, q))
	return app
}

func TestRootRoute(t *testing.T) {
	app := setupApp(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.MethodGet, resp.StatusCode)
	defer resp.Body.Close()
}

func TestIngestEndpoint(t *testing.T) {
	app := setupApp(t)
	body := `{"sourceContext":"local","signals":[{"eventType":"log","source":"svc","severity":5}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	defer resp.Body.Close()
}
