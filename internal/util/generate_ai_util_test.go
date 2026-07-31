package util

import (
	"context"
	"os"
	"testing"
	"time"
	"tracemind/internal/models"

	"github.com/openai/openai-go"
	"github.com/stretchr/testify/assert"
)

func TestGenerateAIPrompt_IncludesUserInputAndInstructions(t *testing.T) {
	t.Parallel()

	userInput := "Incident id=inc-001"
	prompt := generateAIPrompt(userInput)

	assert.NotNil(t, prompt)
	assert.Equal(t, "system", prompt.role)
	assert.Contains(t, prompt.content, "You are TraceMind, an incident analysis assistant")
	assert.Contains(t, prompt.content, "Return valid JSON only")
	assert.Contains(t, prompt.content, userInput)
}

func TestProcessAIGenratedData_SuccessWithJSONCodeFence(t *testing.T) {
	t.Parallel()

	choices := []openai.ChatCompletionChoice{
		{
			Message: openai.ChatCompletionMessage{
				Content: "```json\n{\n  \"hypothesisTemplate\": \"Database connection pool exhausted\",\n  \"recommendations\": [\"Increase pool size\", \"Tune query timeout\"]\n}\n```",
			},
		},
	}

	out, err := processAIGenratedData(context.Background(), choices)
	assert.NoError(t, err)
	assert.NotNil(t, out)
	assert.Equal(t, "Database connection pool exhausted", out.HypothesisTemplate)
	assert.Equal(t, []string{"Increase pool size", "Tune query timeout"}, out.Recommendations)
}

func TestProcessAIGenratedData_FiltersBlankRecommendations(t *testing.T) {
	t.Parallel()

	choices := []openai.ChatCompletionChoice{
		{
			Message: openai.ChatCompletionMessage{
				Content: `{
					"hypothesisTemplate": "Worker backlog caused delayed processing",
					"recommendations": ["  drain backlog ", "", "   "]
				}`,
			},
		},
	}

	out, err := processAIGenratedData(context.Background(), choices)
	assert.NoError(t, err)
	assert.Equal(t, []string{"drain backlog"}, out.Recommendations)
}

func TestProcessAIGenratedData_ErrorCases(t *testing.T) {
	t.Parallel()

	t.Run("no choices", func(t *testing.T) {
		t.Parallel()

		out, err := processAIGenratedData(context.Background(), nil)
		assert.Nil(t, out)
		assert.EqualError(t, err, "no AI response returned")
	})

	t.Run("empty content", func(t *testing.T) {
		t.Parallel()

		choices := []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: ""}}}
		out, err := processAIGenratedData(context.Background(), choices)
		assert.Nil(t, out)
		assert.EqualError(t, err, "AI response content is empty")
	})

	t.Run("invalid json", func(t *testing.T) {
		t.Parallel()

		choices := []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: "not-json"}}}
		out, err := processAIGenratedData(context.Background(), choices)
		assert.Nil(t, out)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse AI response JSON")
	})

	t.Run("missing hypothesis", func(t *testing.T) {
		t.Parallel()

		choices := []openai.ChatCompletionChoice{
			{Message: openai.ChatCompletionMessage{Content: `{"hypothesisTemplate":"  ","recommendations":["step"]}`}},
		}
		out, err := processAIGenratedData(context.Background(), choices)
		assert.Nil(t, out)
		assert.EqualError(t, err, "AI response missing hypothesisTemplate")
	})

	t.Run("missing recommendations", func(t *testing.T) {
		t.Parallel()

		choices := []openai.ChatCompletionChoice{
			{Message: openai.ChatCompletionMessage{Content: `{"hypothesisTemplate":"something","recommendations":["","  "]}`}},
		}
		out, err := processAIGenratedData(context.Background(), choices)
		assert.Nil(t, out)
		assert.EqualError(t, err, "AI response missing recommendations")
	})
}

func TestGenerateAI_ReturnsErrorWhenAPIKeyMissing(t *testing.T) {
	t.Parallel()

	previousValue, hadEnv := os.LookupEnv("OPENAI_API_KEY")
	assert.NoError(t, os.Unsetenv("OPENAI_API_KEY"))
	t.Cleanup(func() {
		if hadEnv {
			assert.NoError(t, os.Setenv("OPENAI_API_KEY", previousValue))
			return
		}
		assert.NoError(t, os.Unsetenv("OPENAI_API_KEY"))
	})

	incident := models.Incident{
		ID:               "inc-123",
		Title:            "API errors spike",
		Status:           "open",
		Severity:         4,
		ImpactedServices: []string{"gateway"},
		Environments:     []string{"prod"},
		AnalysisSummary:  "",
		Recommendations:  nil,
	}

	evidence := []models.Signal{
		{
			ID:        "sig-1",
			EventType: "http",
			Source:    "gateway",
			Env:       "prod",
			Severity:  4,
			Timestamp: time.Now().UTC(),
			Message:   "5xx threshold breached",
		},
	}

	out, err := GenerateAI(context.Background(), incident, evidence)
	assert.Nil(t, out)
	assert.EqualError(t, err, "The OPENAI_API_KEY is not configured on the server")
}
