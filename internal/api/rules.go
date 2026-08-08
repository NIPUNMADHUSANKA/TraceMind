package api

import (
	"database/sql"
	"strings"
	"tracemind/internal/models"
	"tracemind/internal/store"
	"tracemind/internal/util"

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

		id, created, err := s.CreateAnalysisRule(rule)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		statusCode := fiber.StatusCreated
		if !created {
			statusCode = fiber.StatusOK
		}

		return c.Status(statusCode).JSON(fiber.Map{"status": "success", "id": id, "created": created})
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
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
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
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
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

		id, created, err := s.CreateAnalysisRulePattern(pattern)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		statusCode := fiber.StatusCreated
		if !created {
			statusCode = fiber.StatusOK
		}

		return c.Status(statusCode).JSON(fiber.Map{"status": "success", "id": id, "created": created})
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
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
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
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "success", "id": id})
	}
}

func parseAnalysisRuleRequest(c *fiber.Ctx) (models.AnalysisRule, error) {
	var req analysisRuleRequest
	if err := c.BodyParser(&req); err != nil {
		return models.AnalysisRule{}, fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if strings.TrimSpace(req.Name) == "" {
		return models.AnalysisRule{}, fiber.NewError(fiber.StatusBadRequest, "name is required")
	}
	if strings.TrimSpace(req.HypothesisTemplate) == "" {
		return models.AnalysisRule{}, fiber.NewError(fiber.StatusBadRequest, "hypothesisTemplate is required")
	}

	priority := 100
	if req.Priority != nil {
		if *req.Priority >= 0 && *req.Priority <= 100 {
			priority = *req.Priority
		} else {
			priority = 100
		}
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	matchType := models.MatchTypeSingle
	if strings.ToLower(strings.TrimSpace(string(req.MatchType))) == string(models.MatchTypeSingle) {
		matchType = models.MatchTypeSingle
	} else if strings.ToLower(strings.TrimSpace(string(req.MatchType))) == string(models.MatchTypeCorrelation) {
		matchType = models.MatchTypeCorrelation
	} else {
		return models.AnalysisRule{}, fiber.NewError(fiber.StatusBadRequest, "The matchType value must be either SINGLE or CORRELATION.")
	}
	version := 1
	if req.Version != nil {
		if *req.Version >= 1 {
			version = *req.Version
		} else {
			return models.AnalysisRule{}, fiber.NewError(fiber.StatusBadRequest, "The version must be at least 1")
		}
	}

	confidence := req.Confidence
	if confidence < 0 || confidence > 1 {
		return models.AnalysisRule{}, fiber.NewError(fiber.StatusBadRequest, "The confidence value must be between 0 and 1")
	}

	return models.AnalysisRule{
		ID:                 strings.TrimSpace(req.ID),
		Name:               strings.TrimSpace(req.Name),
		Description:        strings.TrimSpace(req.Description),
		Confidence:         confidence,
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
		if strings.Contains(strings.ToLower(err.Error()), "unknown field") {
			return models.AnalysisRulePattern{}, fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
		return models.AnalysisRulePattern{}, fiber.NewError(fiber.StatusBadRequest, err.Error())
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

	eventType := strings.TrimSpace(req.EventType)
	if eventType == "" {
		return models.AnalysisRulePattern{}, fiber.NewError(fiber.StatusBadRequest, "The EventType value is required")
	}

	source := strings.TrimSpace(req.Source)
	if source == "" {
		return models.AnalysisRulePattern{}, fiber.NewError(fiber.StatusBadRequest, "The source value is required")
	}

	environment := strings.TrimSpace(req.Environment)
	if environment == "" {
		return models.AnalysisRulePattern{}, fiber.NewError(fiber.StatusBadRequest, "The Environment value is required")
	}

	environment, err := util.FormatEnvironment(environment)
	if err != nil {
		return models.AnalysisRulePattern{}, fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if req.SeverityMin == nil {
		return models.AnalysisRulePattern{}, fiber.NewError(fiber.StatusBadRequest, "The SeverityMin value is required")
	}

	severity := *req.SeverityMin
	if severity < 0 || severity > 100 {
		return models.AnalysisRulePattern{}, fiber.NewError(fiber.StatusBadRequest, "The SeverityMin value must be between 0 and 100")
	}

	var MessageMatchType models.MessageMatchType
	if strings.ToLower(strings.TrimSpace(string(req.MessageMatchType))) == string(models.MessageMatchExact) {
		MessageMatchType = models.MessageMatchExact
	} else if strings.ToLower(strings.TrimSpace(string(req.MessageMatchType))) == string(models.MessageMatchRegex) {
		MessageMatchType = models.MessageMatchRegex
	} else if strings.ToLower(strings.TrimSpace(string(req.MessageMatchType))) == string(models.MessageMatchContains) {
		MessageMatchType = models.MessageMatchContains
	} else {
		return models.AnalysisRulePattern{}, fiber.NewError(fiber.StatusBadRequest, "The MessageMatchType value must be either EXACT, CONTAINS, or REGEX")
	}

	messagePattern := strings.TrimSpace(req.MessagePattern)
	if messagePattern == "" {
		return models.AnalysisRulePattern{}, fiber.NewError(fiber.StatusBadRequest, "The MessagePattern value is required")
	}

	return models.AnalysisRulePattern{
		ID:                strings.TrimSpace(req.ID),
		RuleID:            strings.TrimSpace(req.RuleID),
		EventType:         eventType,
		Source:            source,
		Environment:       environment,
		SeverityMin:       req.SeverityMin,
		MessageMatchType:  MessageMatchType,
		MessagePattern:    messagePattern,
		PayloadConditions: req.PayloadConditions,
		VariableMappings:  req.VariableMappings,
	}, nil
}
