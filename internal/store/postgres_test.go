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

func TestPostgresStore_Close(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	mock.ExpectClose()

	ps := &PostgresStore{db: db}
	require.NoError(t, ps.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}
