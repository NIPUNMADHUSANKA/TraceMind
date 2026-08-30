package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"tracemind/internal/api"
	"tracemind/internal/store"
	"tracemind/internal/util"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestSSEHandlerRejectsMissingIngestionID(t *testing.T) {
	app := fiber.New()
	app.Get("/events", api.HandleSSE(store.PostgresStore{}, util.NewBroker()))

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/events", nil))

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.NoError(t, resp.Body.Close())
}
