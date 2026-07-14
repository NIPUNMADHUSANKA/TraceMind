package api

import (
	"strings"
	"tracemind/internal/store"

	"github.com/gofiber/fiber/v2"
)

type PayloadFilterRequestInput struct {
	Payloads []string `json:"payloads"`
}

func PayloadFilter(s store.PostgresStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		env := c.Params("environment")
		if env == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "environment path parameter is required"})
		}

		var req PayloadFilterRequestInput

		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "request body must be valid JSON"})
		}

		payloadList := make([]string, 0, len(req.Payloads))
		seen := make(map[string]bool, len(req.Payloads))
		for _, val := range req.Payloads {
			key := strings.TrimSpace(val)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			payloadList = append(payloadList, key)
		}

		if len(payloadList) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "payloads must contain at least one key"})
		}

		if err := s.SavePayloadFilterConfig(env, payloadList); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update payload allow-list"})
		}

		store.ConfigurePayloadAllowList(s, env)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":      "success",
			"message":     "payload allow-list updated",
			"environment": env,
			"count":       len(payloadList),
		})

	}
}
