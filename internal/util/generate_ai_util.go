package util

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"tracemind/internal/models"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type ChatMessage struct {
	role    string
	content string
}

var MODEL_NAME = openai.ChatModelGPT4_1Mini

func generateAIPrompt(userPrompt string) *ChatMessage {
	prompt := fmt.Sprintf(`You are TraceMind, an incident analysis assistant.

You will receive a structured incident payload with these sections:
- Incident details
- Impacted services and environments
- Existing summary and recommendations
- Evidence signals

Tasks:
1. Infer the most likely root-cause hypothesis from incident details and evidence.
2. Produce actionable recommendations in priority order.
3. Use only the provided information.

Response rules:
- Return valid JSON only.
- JSON schema:
  {
    "hypothesisTemplate": "string",
    "recommendations": ["string"]
  }
- Do not add markdown or extra keys.
- If evidence is insufficient, state uncertainty in hypothesisTemplate and still provide best next-step recommendations.

User input:
%s`, userPrompt)

	messages := &ChatMessage{
		role:    "system",
		content: prompt,
	}

	return messages
}

func GenerateAI(ctx context.Context, incident models.Incident, evidence []models.Signal) (*models.AiSuggestion, error) {

	listOrNone := func(items []string) string {
		if len(items) == 0 {
			return "none"
		}

		return strings.Join(items, ", ")
	}

	var evidenceLines strings.Builder
	if len(evidence) == 0 {
		evidenceLines.WriteString("- none")
	} else {
		for i, signal := range evidence {
			timestamp := "unknown"
			if !signal.Timestamp.IsZero() {
				timestamp = signal.Timestamp.UTC().Format("2006-01-02T15:04:05Z")
			}

			evidenceLines.WriteString(fmt.Sprintf(
				"- #%d id=%s eventType=%s source=%s env=%s severity=%d timestamp=%s message=%s\n",
				i+1,
				signal.ID,
				signal.EventType,
				signal.Source,
				signal.Env,
				signal.Severity,
				timestamp,
				signal.Message,
			))
		}
	}

	userInput := fmt.Sprintf(`Incident:
- id: %s
- title: %s
- status: %s
- severity: %d
- impactedServices: %s
- environments: %s
- existingAnalysisSummary: %s
- existingRecommendations: %s

Evidence (%d):
%s`,
		incident.ID,
		incident.Title,
		incident.Status,
		incident.Severity,
		listOrNone(incident.ImpactedServices),
		listOrNone(incident.Environments),
		strings.TrimSpace(incident.AnalysisSummary),
		listOrNone(incident.Recommendations),
		len(evidence),
		strings.TrimSpace(evidenceLines.String()),
	)

	chatMessage := generateAIPrompt(userInput)

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("The OPENAI_API_KEY is not configured on the server")
	}

	if chatMessage == nil {
		return nil, errors.New("prompt cannot be generated, please contact the system administrator")
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
	)

	resp, err := client.Chat.Completions.New(
		ctx,
		openai.ChatCompletionNewParams{
			Model: MODEL_NAME,
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.SystemMessage(chatMessage.content),
			},
		},
	)

	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "insufficient_quota") {
			return nil, errors.New("OpenAI quota exceeded: please add credits at https://platform.openai.com/settings/billing")
		}
		if strings.Contains(errMsg, "429") {
			return nil, errors.New("OpenAI rate limit reached: too many requests, please try again later")
		}
		if strings.Contains(errMsg, "401") {
			return nil, errors.New("OpenAI authentication failed: the API key is invalid or expired")
		}
		return nil, err
	}

	return processAIGenratedData(ctx, resp.Choices)
}

func processAIGenratedData(ctx context.Context, result []openai.ChatCompletionChoice) (*models.AiSuggestion, error) {
	_ = ctx

	if len(result) == 0 {
		return nil, errors.New("no AI response returned")
	}

	aiText := result[0].Message.Content
	if aiText == "" {
		return nil, errors.New("AI response content is empty")
	}

	type aiSuggestionResponse struct {
		HypothesisTemplate string   `json:"hypothesisTemplate"`
		Recommendations    []string `json:"recommendations"`
	}

	clean := strings.TrimSpace(aiText)
	if strings.HasPrefix(clean, "```") {
		clean = strings.TrimPrefix(clean, "```json")
		clean = strings.TrimPrefix(clean, "```")
		clean = strings.TrimSuffix(clean, "```")
		clean = strings.TrimSpace(clean)
	}

	var parsed aiSuggestionResponse
	if err := json.Unmarshal([]byte(clean), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse AI response JSON: %w", err)
	}

	parsed.HypothesisTemplate = strings.TrimSpace(parsed.HypothesisTemplate)
	if parsed.HypothesisTemplate == "" {
		return nil, errors.New("AI response missing hypothesisTemplate")
	}

	mappedRecommendations := make([]string, 0, len(parsed.Recommendations))
	for _, recommendation := range parsed.Recommendations {
		item := strings.TrimSpace(recommendation)
		if item != "" {
			mappedRecommendations = append(mappedRecommendations, item)
		}
	}

	if len(mappedRecommendations) == 0 {
		return nil, errors.New("AI response missing recommendations")
	}

	return &models.AiSuggestion{
		HypothesisTemplate: parsed.HypothesisTemplate,
		Recommendations:    mappedRecommendations,
	}, nil

}
