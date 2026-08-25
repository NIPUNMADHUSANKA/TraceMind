package api

import (
	"context"
	"tracemind/internal/queue"

	"github.com/gofiber/fiber/v2"
)

type queueStatsProvider interface {
	Health(context.Context) (*queue.QueueHealth, error)
}

func HealthHandler(q queueStatsProvider) fiber.Handler {
	return func(c *fiber.Ctx) error {
		stats, err := q.Health(context.Background())
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{
			"status":    "healthy",
			"available": stats.Available,
			"inFlight":  stats.InFlight,
			"delayed":   stats.Delayed,
		})
	}
}
