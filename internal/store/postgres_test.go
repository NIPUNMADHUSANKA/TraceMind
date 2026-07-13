package store

import (
	"database/sql/driver"
	"testing"
	"time"
	"tracemind/internal/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

type nonEmptyStringMatcher struct{}

func (m nonEmptyStringMatcher) Match(v driver.Value) bool {
	s, ok := v.(string)
	return ok && s != ""
}

type nonZeroTimeMatcher struct{}

func (m nonZeroTimeMatcher) Match(v driver.Value) bool {
	t, ok := v.(time.Time)
	return ok && !t.IsZero()
}

func TestPostgresStore_SaveSignal_SetsDefaults(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ps := &PostgresStore{db: db}

	mock.ExpectExec("INSERT INTO signals").
		WithArgs(
			nonEmptyStringMatcher{},
			"log",
			"svc",
			"prod",
			nonZeroTimeMatcher{},
			5,
			"m",
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	ps.SaveSignal(models.Signal{
		EventType: "log",
		Source:    "svc",
		Env:       "prod",
		Severity:  5,
		Message:   "m",
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStore_SaveSignal_SanitizesMessage(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ps := &PostgresStore{db: db}

	mock.ExpectExec("INSERT INTO signals").
		WithArgs(
			nonEmptyStringMatcher{},
			"log",
			"svc",
			"prod",
			nonZeroTimeMatcher{},
			2,
			"contact=[REDACTED_EMAIL] password=[REDACTED]",
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	ps.SaveSignal(models.Signal{
		EventType: "log",
		Source:    "svc",
		Env:       "prod",
		Severity:  2,
		Message:   "contact=alice@example.com password=supersecret",
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStore_SaveIncident_SetsDefaultTimes(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ps := &PostgresStore{db: db}

	mock.ExpectExec("INSERT INTO incidents").
		WithArgs(
			nonEmptyStringMatcher{},
			"incident",
			"new",
			4,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			"",
			sqlmock.AnyArg(),
			nonZeroTimeMatcher{},
			nonZeroTimeMatcher{},
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	ps.SaveIncident(models.Incident{
		Title:    "incident",
		Status:   "new",
		Severity: 4,
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStore_UpdateIncidentStatus_UpdatesOnlyStatus(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ps := &PostgresStore{db: db}

	mock.ExpectExec("UPDATE incidents SET status = \\$2, updated_at = \\$3 WHERE id = \\$1").
		WithArgs("inc-1", "In-Progress", nonZeroTimeMatcher{}).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = ps.UpdateIncidentStatus("inc-1", "In-Progress")
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStore_UpdateIncidentStatus_ValidatesInput(t *testing.T) {
	t.Parallel()

	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ps := &PostgresStore{db: db}

	err = ps.UpdateIncidentStatus("", "In-Progress")
	require.Error(t, err)

	err = ps.UpdateIncidentStatus("inc-1", "")
	require.Error(t, err)
}

func TestPostgresStore_GetSignal_InvalidJSON_ReturnsFalse(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ps := &PostgresStore{db: db}
	now := time.Now().UTC()

	rows := sqlmock.NewRows([]string{"id", "event_type", "source", "env", "timestamp", "severity", "message", "payload", "metadata"}).
		AddRow("id-1", "log", "svc", "prod", now, 3, "msg", "{bad-json", `{"k":"v"}`)

	mock.ExpectQuery("SELECT id, event_type, source, env, timestamp, severity, message, payload, metadata FROM signals WHERE id = \\$1").
		WithArgs("id-1").
		WillReturnRows(rows)

	_, ok := ps.GetSignal("id-1")
	require.False(t, ok)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStore_DeleteSignalsOlderThan_DeletesInBatches(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ps := &PostgresStore{db: db}
	cutoff := time.Now().UTC().Add(-24 * time.Hour)

	mock.ExpectExec("DELETE FROM signals").
		WithArgs(cutoff, signalDeleteBatchSize).
		WillReturnResult(sqlmock.NewResult(0, signalDeleteBatchSize))
	mock.ExpectExec("DELETE FROM signals").
		WithArgs(cutoff, signalDeleteBatchSize).
		WillReturnResult(sqlmock.NewResult(0, 250))

	deleted := ps.DeleteSignalsOlderThan(cutoff)
	require.Equal(t, signalDeleteBatchSize+250, deleted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStore_DeleteIncidentsOlderThan_DeletesInBatches(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ps := &PostgresStore{db: db}
	cutoff := time.Now().UTC().Add(-24 * time.Hour)

	mock.ExpectExec("DELETE FROM incidents").
		WithArgs(cutoff, signalDeleteBatchSize).
		WillReturnResult(sqlmock.NewResult(0, signalDeleteBatchSize))
	mock.ExpectExec("DELETE FROM incidents").
		WithArgs(cutoff, signalDeleteBatchSize).
		WillReturnResult(sqlmock.NewResult(0, 250))

	deleted := ps.DeleteIncidentsOlderThan(cutoff)
	require.Equal(t, signalDeleteBatchSize+250, deleted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStore_ListIncidents_DecodesRows(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ps := &PostgresStore{db: db}
	now := time.Now().UTC()

	rows := sqlmock.NewRows([]string{"id", "title", "status", "severity", "impacted_services", "environments", "signal_ids", "analysis_summary", "recommendations", "created_at", "updated_at"}).
		AddRow(
			"inc-1",
			"Auto-generated incident",
			"open",
			5,
			[]byte(`["svc-a","svc-b"]`),
			[]byte(`["prod"]`),
			[]byte(`["sig-1","sig-2"]`),
			"summary",
			[]byte(`["do x","do y"]`),
			now,
			now.Add(time.Minute),
		)

	mock.ExpectQuery("SELECT id, title, status, severity, impacted_services, environments, signal_ids, analysis_summary, recommendations, created_at, updated_at FROM incidents").
		WillReturnRows(rows)

	incidents := ps.ListIncidents()
	require.Len(t, incidents, 1)
	require.Equal(t, "inc-1", incidents[0].ID)
	require.Equal(t, "Auto-generated incident", incidents[0].Title)
	require.Equal(t, "open", incidents[0].Status)
	require.Equal(t, 5, incidents[0].Severity)
	require.ElementsMatch(t, []string{"svc-a", "svc-b"}, incidents[0].ImpactedServices)
	require.ElementsMatch(t, []string{"prod"}, incidents[0].Environments)
	require.ElementsMatch(t, []string{"sig-1", "sig-2"}, incidents[0].SignalIDs)
	require.Equal(t, "summary", incidents[0].AnalysisSummary)
	require.ElementsMatch(t, []string{"do x", "do y"}, incidents[0].Recommendations)
	require.Equal(t, now, incidents[0].CreatedAt)
	require.Equal(t, now.Add(time.Minute), incidents[0].UpdatedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStore_GetIncident_DecodesRow(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ps := &PostgresStore{db: db}
	now := time.Now().UTC()

	rows := sqlmock.NewRows([]string{"id", "title", "status", "severity", "impacted_services", "environments", "signal_ids", "analysis_summary", "recommendations", "created_at", "updated_at"}).
		AddRow(
			"inc-1",
			"Auto-generated incident",
			"open",
			4,
			[]byte(`["svc-a"]`),
			[]byte(`["prod"]`),
			[]byte(`["sig-9"]`),
			"summary",
			[]byte(`["do x"]`),
			now,
			now.Add(2*time.Minute),
		)

	mock.ExpectQuery("SELECT id, title, status, severity, impacted_services, environments, signal_ids, analysis_summary, recommendations, created_at, updated_at FROM incidents WHERE id = \\$1").
		WithArgs("inc-1").
		WillReturnRows(rows)

	inc, ok := ps.GetIncident("inc-1")
	require.True(t, ok)
	require.Equal(t, "inc-1", inc.ID)
	require.Equal(t, "Auto-generated incident", inc.Title)
	require.Equal(t, "open", inc.Status)
	require.Equal(t, 4, inc.Severity)
	require.ElementsMatch(t, []string{"svc-a"}, inc.ImpactedServices)
	require.ElementsMatch(t, []string{"prod"}, inc.Environments)
	require.ElementsMatch(t, []string{"sig-9"}, inc.SignalIDs)
	require.Equal(t, "summary", inc.AnalysisSummary)
	require.ElementsMatch(t, []string{"do x"}, inc.Recommendations)
	require.Equal(t, now, inc.CreatedAt)
	require.Equal(t, now.Add(2*time.Minute), inc.UpdatedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresStore_Close(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	mock.ExpectClose()

	ps := &PostgresStore{db: db}
	require.NoError(t, ps.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}
