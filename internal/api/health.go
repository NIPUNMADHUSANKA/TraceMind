package api

import (
	"tracemind/internal/queue"
	"tracemind/internal/store"

	"github.com/gofiber/fiber/v2"
)

func HealthHandler(q chan queue.IngestionJob, s store.PostgresStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		pending := len(q)
		incCount := len(s.ListIncidents())
		return c.JSON(fiber.Map{
			"ingestion": fiber.Map{"queueDepth": pending},
			"incidents": incCount,
		})
	}
}
