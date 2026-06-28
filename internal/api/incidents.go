package api

import (
	"tracemind/internal/store"

	"github.com/gofiber/fiber/v2"
)

func IncidentsHandler(s store.PostgresStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		list := s.ListIncidents()
		return c.JSON(fiber.Map{
			"incidents": list,
		})
	}
}

func IncidentGetHandler(s store.PostgresStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing id"})
		}
		if inc, ok := s.GetIncident(id); ok {
			return c.JSON(inc)
		}
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}
}
