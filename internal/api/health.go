package api

import (
	"tracemind/internal/queue"
	"tracemind/internal/store"

	"github.com/gofiber/fiber/v2"
)

type queueStatsProvider interface {
	Stats() queue.QueueStats
}

func HealthHandler(q queueStatsProvider, s store.PostgresStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		stats := q.Stats()
		incCount := len(s.ListIncidents())
		return c.JSON(fiber.Map{
			"ingestion": fiber.Map{
				"queueDepth":             stats.Depth,
				"retryCount":             stats.RetryCount,
				"deadLetterCount":        stats.DeadLetterCount,
				"lastProcessedTimestamp": stats.LastProcessedTimestamp,
			},
			"incidents": incCount,
		})
	}
}
