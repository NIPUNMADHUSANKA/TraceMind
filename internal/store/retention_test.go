package store

import (
	"testing"
	"time"
	"tracemind/internal/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
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

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ps := PostgresStore{db: db}
	mock.ExpectExec("DELETE FROM signals").
		WithArgs(sqlmock.AnyArg(), signalDeleteBatchSize).
		WillReturnResult(sqlmock.NewResult(0, 1))

	stop := make(chan struct{})
	startRetentionEnforcerWithInterval(ps, "signals", time.Hour, time.Hour, stop)
	t.Cleanup(func() { close(stop) })

	require.Eventually(t, func() bool {
		return mock.ExpectationsWereMet() == nil
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

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ps := PostgresStore{db: db}
	rows := sqlmock.NewRows([]string{"allow_payload"}).
		AddRow("requestId")
	mock.ExpectQuery("SELECT allow_payload FROM payload_filter_configs WHERE environment = \\$1 ORDER BY allow_payload ASC").
		WithArgs("staging").
		WillReturnRows(rows)

	ConfigurePayloadAllowList(ps, "staging")
	t.Cleanup(func() {
		payloadAllowListMu.Lock()
		payloadAllowList = nil
		payloadAllowListMu.Unlock()
	})

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
	require.NoError(t, mock.ExpectationsWereMet())
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
