package api

import (
	"database/sql"
	"strings"
	"tracemind/internal/models"
	"tracemind/internal/store"

	"github.com/gofiber/fiber/v2"
)

type analysisRuleRequest struct {
	ID                 string               `json:"id,omitempty"`
	Name               string               `json:"name"`
	Description        string               `json:"description"`
	Confidence         float64              `json:"confidence"`
	Priority           *int                 `json:"priority"`
	Enabled            *bool                `json:"enabled"`
	MatchType          models.RuleMatchType `json:"matchType"`
	HypothesisTemplate string               `json:"hypothesisTemplate"`
	Recommendations    []string             `json:"recommendations"`
	Version            *int                 `json:"version"`
}

type analysisRulePatternRequest struct {
	ID                string                    `json:"id,omitempty"`
	RuleID            string                    `json:"ruleId"`
	EventType         string                    `json:"eventType"`
	Source            string                    `json:"source"`
	Environment       string                    `json:"environment"`
	SeverityMin       *int                      `json:"severityMin"`
	MessageMatchType  models.MessageMatchType   `json:"messageMatchType"`
	MessagePattern    string                    `json:"messagePattern"`
	PayloadConditions []models.PayloadCondition `json:"payloadConditions"`
	VariableMappings  map[string]string         `json:"variableMappings"`
}

func CreateAnalysisRuleHandler(s store.PostgresStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rule, err := parseAnalysisRuleRequest(c)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		id, err := s.CreateAnalysisRule(rule)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create analysis rule"})
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "id": id})
	}
}

func UpdateAnalysisRuleHandler(s store.PostgresStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := strings.TrimSpace(c.Params("id"))
		if id == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "analysis rule id path parameter is required"})
		}

		rule, err := parseAnalysisRuleRequest(c)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		err = s.UpdateAnalysisRule(id, rule)
		if err != nil {
			if err == sql.ErrNoRows {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "analysis rule not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update analysis rule"})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "id": id})
	}
}

func DeleteAnalysisRuleHandler(s store.PostgresStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := strings.TrimSpace(c.Params("id"))
		if id == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "analysis rule id path parameter is required"})
		}

		err := s.DeleteAnalysisRule(id)
		if err != nil {
			if err == sql.ErrNoRows {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "analysis rule not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete analysis rule"})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "id": id})
	}
}

func CreateAnalysisRulePatternHandler(s store.PostgresStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		pattern, err := parseAnalysisRulePatternRequest(c)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		id, err := s.CreateAnalysisRulePattern(pattern)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create analysis rule pattern"})
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "id": id})
	}
}

func UpdateAnalysisRulePatternHandler(s store.PostgresStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := strings.TrimSpace(c.Params("id"))
		if id == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "analysis rule pattern id path parameter is required"})
		}

		pattern, err := parseAnalysisRulePatternRequest(c)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		err = s.UpdateAnalysisRulePattern(id, pattern)
		if err != nil {
			if err == sql.ErrNoRows {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "analysis rule pattern not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update analysis rule pattern"})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "id": id})
	}
}

func DeleteAnalysisRulePatternHandler(s store.PostgresStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := strings.TrimSpace(c.Params("id"))
		if id == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "analysis rule pattern id path parameter is required"})
		}

		err := s.DeleteAnalysisRulePattern(id)
		if err != nil {
			if err == sql.ErrNoRows {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "analysis rule pattern not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete analysis rule pattern"})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "id": id})
	}
}

func parseAnalysisRuleRequest(c *fiber.Ctx) (models.AnalysisRule, error) {
	var req analysisRuleRequest
	if err := c.BodyParser(&req); err != nil {
		return models.AnalysisRule{}, fiber.NewError(fiber.StatusBadRequest, "request body must be valid JSON")
	}
	if strings.TrimSpace(req.Name) == "" {
		return models.AnalysisRule{}, fiber.NewError(fiber.StatusBadRequest, "name is required")
	}
	if strings.TrimSpace(req.HypothesisTemplate) == "" {
		return models.AnalysisRule{}, fiber.NewError(fiber.StatusBadRequest, "hypothesisTemplate is required")
	}

	priority := 100
	if req.Priority != nil {
		priority = *req.Priority
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	matchType := models.MatchTypeSingle
	if strings.TrimSpace(string(req.MatchType)) == string(models.MatchTypeSingle) {
		matchType = models.MatchTypeSingle
	} else if strings.TrimSpace(string(req.MatchType)) == string(models.MatchTypeCorrelation) {
		matchType = models.MatchTypeCorrelation
	}
	version := 1
	if req.Version != nil {
		version = *req.Version
	}

	return models.AnalysisRule{
		ID:                 strings.TrimSpace(req.ID),
		Name:               strings.TrimSpace(req.Name),
		Description:        strings.TrimSpace(req.Description),
		Confidence:         req.Confidence,
		Priority:           priority,
		Enabled:            enabled,
		MatchType:          matchType,
		HypothesisTemplate: strings.TrimSpace(req.HypothesisTemplate),
		Recommendations:    req.Recommendations,
		Version:            version,
	}, nil
}

func parseAnalysisRulePatternRequest(c *fiber.Ctx) (models.AnalysisRulePattern, error) {
	var req analysisRulePatternRequest
	if err := c.BodyParser(&req); err != nil {
		return models.AnalysisRulePattern{}, fiber.NewError(fiber.StatusBadRequest, "request body must be valid JSON")
	}
	if strings.TrimSpace(req.RuleID) == "" {
		return models.AnalysisRulePattern{}, fiber.NewError(fiber.StatusBadRequest, "ruleId is required")
	}
	if (strings.TrimSpace(string(req.MessageMatchType)) == "") != (strings.TrimSpace(req.MessagePattern) == "") {
		return models.AnalysisRulePattern{}, fiber.NewError(fiber.StatusBadRequest, "messageMatchType and messagePattern must be provided together")
	}
	if req.PayloadConditions == nil {
		req.PayloadConditions = []models.PayloadCondition{}
	}
	if req.VariableMappings == nil {
		req.VariableMappings = map[string]string{}
	}

	MessageMatchType := models.MessageMatchContains
	if strings.TrimSpace(string(req.MessageMatchType)) == string(models.MessageMatchExact) {
		MessageMatchType = models.MessageMatchExact
	} else if strings.TrimSpace(string(req.MessageMatchType)) == string(models.MessageMatchRegex) {
		MessageMatchType = models.MessageMatchRegex
	}

	return models.AnalysisRulePattern{
		ID:                strings.TrimSpace(req.ID),
		RuleID:            strings.TrimSpace(req.RuleID),
		EventType:         strings.TrimSpace(req.EventType),
		Source:            strings.TrimSpace(req.Source),
		Environment:       strings.TrimSpace(req.Environment),
		SeverityMin:       req.SeverityMin,
		MessageMatchType:  MessageMatchType,
		MessagePattern:    strings.TrimSpace(req.MessagePattern),
		PayloadConditions: req.PayloadConditions,
		VariableMappings:  req.VariableMappings,
	}, nil
}
