package api

import (
	"strings"
	"tracemind/internal/store"

	"github.com/gofiber/fiber/v2"
)

type PayloadFilterRequestInput struct {
	Payloads []string `json:"payloads"`
}

func normalizePayloadList(values []string) []string {
	payloadList := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, val := range values {
		key := strings.TrimSpace(val)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		payloadList = append(payloadList, key)
	}
	return payloadList
}

func parsePayloadFilterRequest(c *fiber.Ctx) ([]string, error) {
	var req PayloadFilterRequestInput

	if err := c.BodyParser(&req); err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "request body must be valid JSON")
	}

	payloadList := normalizePayloadList(req.Payloads)
	if len(payloadList) == 0 {
		return nil, fiber.NewError(fiber.StatusBadRequest, "payloads must contain at least one key")
	}

	return payloadList, nil
}

func PayloadFilter(s store.PostgresStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		env := c.Params("environment")
		if env == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "environment path parameter is required"})
		}

		payloadList, err := parsePayloadFilterRequest(c)
		if err != nil {
			if e, ok := err.(*fiber.Error); ok {
				return c.Status(e.Code).JSON(fiber.Map{"error": e.Message})
			}
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "request body must be valid JSON"})
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

func DeletePayloadFilter(s store.PostgresStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		env := c.Params("environment")
		if env == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "environment path parameter is required"})
		}

		payloadList, err := parsePayloadFilterRequest(c)
		if err != nil {
			if e, ok := err.(*fiber.Error); ok {
				return c.Status(e.Code).JSON(fiber.Map{"error": e.Message})
			}
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "request body must be valid JSON"})
		}

		deleted, err := s.DeletePayloadFilterConfig(env, payloadList)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update payload allow-list"})
		}

		store.ConfigurePayloadAllowList(s, env)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":      "success",
			"message":     "payload allow-list updated",
			"environment": env,
			"count":       deleted,
		})
	}
}
