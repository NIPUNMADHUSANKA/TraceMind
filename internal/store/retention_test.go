package store

import (
	"testing"
	"time"
	"tracemind/internal/models"

	"github.com/stretchr/testify/require"
)

func TestDeleteSignalsOlderThan_RemovesOnlyExpired(t *testing.T) {
	t.Parallel()

	s := NewStore()
	now := time.Now().UTC()

	s.SaveSignal(models.Signal{ID: "old-1", EventType: "log", Source: "svc", Timestamp: now.Add(-48 * time.Hour), Severity: 2})
	s.SaveSignal(models.Signal{ID: "new-1", EventType: "log", Source: "svc", Timestamp: now.Add(-2 * time.Hour), Severity: 2})

	deleted := s.DeleteSignalsOlderThan(now.Add(-24 * time.Hour))
	require.Equal(t, 1, deleted)

	_, okOld := s.GetSignal("old-1")
	_, okNew := s.GetSignal("new-1")
	require.False(t, okOld)
	require.True(t, okNew)
}

func TestStartRetentionEnforcer_DeletesOnTicker(t *testing.T) {
	t.Parallel()

	s := NewStore()
	now := time.Now().UTC()
	s.SaveSignal(models.Signal{ID: "old-2", EventType: "log", Source: "svc", Timestamp: now.Add(-2 * time.Hour), Severity: 2})

	stop := make(chan struct{})
	startRetentionEnforcerWithInterval(s, time.Hour, 10*time.Millisecond, stop)
	t.Cleanup(func() { close(stop) })

	require.Eventually(t, func() bool {
		_, ok := s.GetSignal("old-2")
		return !ok
	}, time.Second, 20*time.Millisecond)
}

func TestRedactPayloadByAllowList(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"requestId": "r1",
		"token":     "secret",
		"email":     "user@example.com",
	}

	allow := map[string]bool{"requestId": true}
	output := RedactPayloadByAllowList(input, allow)

	require.Equal(t, "r1", output["requestId"])
	_, hasToken := output["token"]
	_, hasEmail := output["email"]
	require.False(t, hasToken)
	require.False(t, hasEmail)
}

func TestConfigurePayloadAllowList_AppliesDuringSaveSignal(t *testing.T) {
	t.Parallel()

	ConfigurePayloadAllowList([]string{"requestId"})
	t.Cleanup(func() { ConfigurePayloadAllowList(nil) })

	s := NewStore()
	s.SaveSignal(models.Signal{
		ID:        "sig-redact",
		EventType: "log",
		Source:    "svc",
		Severity:  3,
		Payload: map[string]interface{}{
			"requestId": "r2",
			"token":     "secret",
		},
	})

	sig, ok := s.GetSignal("sig-redact")
	require.True(t, ok)
	require.Equal(t, "r2", sig.Payload["requestId"])
	_, hasToken := sig.Payload["token"]
	require.False(t, hasToken)
}

func TestSaveSignal_SanitizesMessage(t *testing.T) {
	t.Parallel()

	s := NewStore()
	s.SaveSignal(models.Signal{
		ID:        "sig-msg-redact",
		EventType: "log",
		Source:    "svc",
		Severity:  2,
		Message:   "user=user@example.com token=abc123",
	})

	sig, ok := s.GetSignal("sig-msg-redact")
	require.True(t, ok)
	require.Equal(t, "user=[REDACTED_EMAIL] token=[REDACTED]", sig.Message)
}
