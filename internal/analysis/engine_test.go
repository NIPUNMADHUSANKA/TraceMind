package analysis

import (
	"reflect"
	"strings"
	"testing"
	"time"
	"tracemind/internal/models"
	"tracemind/internal/store"
	"unsafe"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func analysisRuleQueryColumns() []string {
	return []string{
		"id",
		"confidence",
		"priority",
		"match_type",
		"hypothesis_template",
		"recommendations",
		"id",
		"rule_id",
		"severity_min",
		"message_match_type",
		"message_pattern",
		"payload_conditions",
		"variable_mappings",
	}
}

func newAnalysisStore(t *testing.T, rows *sqlmock.Rows) store.PostgresStore {
	t.Helper()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	mock.ExpectQuery("FROM analysis_rules r").
		WithArgs(sqlmock.AnyArg(), "", "").
		WillReturnRows(rows)

	t.Cleanup(func() {
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})

	engineStore := store.PostgresStore{}
	rv := reflect.ValueOf(&engineStore).Elem().FieldByName("db")
	require.True(t, rv.IsValid())
	require.True(t, rv.CanAddr())
	require.True(t, rv.CanSet() || rv.CanAddr())
	reflect.NewAt(rv.Type(), unsafe.Pointer(rv.UnsafeAddr())).Elem().Set(reflect.ValueOf(db))

	return engineStore
}

func TestRuleEngine_DatabaseFailurePattern(t *testing.T) {
	t.Parallel()

	engine := NewRuleEngine()
	rows := sqlmock.NewRows(analysisRuleQueryColumns()).
		AddRow(
			"rule-db",
			0.9,
			100,
			"single",
			"database bottleneck",
			[]byte(`["inspect DB connections"]`),
			"pat-db",
			"rule-db",
			5,
			"contains",
			"too many connections",
			[]byte(`[]`),
			[]byte(`{}`),
		)
	store := newAnalysisStore(t, rows)
	result := engine.Analyze(models.Incident{ID: "inc-db"}, []models.Signal{
		{EventType: "database", Severity: 5, Message: "too many connections"},
		{EventType: "health", Severity: 4, Message: "service timeout"},
	}, store)

	require.Contains(t, strings.Join(result.Hypotheses, " "), "database bottleneck")
	require.NotEmpty(t, result.Recommendations)
	require.Equal(t, "rule-based", result.Source)
}

func TestRuleEngine_DeploymentOutagePattern(t *testing.T) {
	t.Parallel()

	engine := NewRuleEngine()
	rows := sqlmock.NewRows(analysisRuleQueryColumns()).
		AddRow(
			"rule-deploy",
			0.82,
			120,
			"single",
			"deployment outage",
			[]byte(`["check rollout status"]`),
			"pat-deploy",
			"rule-deploy",
			3,
			"contains",
			"release completed",
			[]byte(`[]`),
			[]byte(`{}`),
		)
	store := newAnalysisStore(t, rows)
	result := engine.Analyze(models.Incident{ID: "inc-deploy"}, []models.Signal{
		{EventType: "deployment", Severity: 4, Message: "release completed 3m ago"},
		{EventType: "health", Severity: 5, Message: "service unavailable"},
	}, store)

	require.Contains(t, strings.Join(result.Hypotheses, " "), "deployment outage")
	require.NotEmpty(t, result.Recommendations)
	require.Equal(t, "rule-based", result.Source)
}

func TestRuleEngine_QueueBacklogPattern(t *testing.T) {
	t.Parallel()

	engine := NewRuleEngine()
	rows := sqlmock.NewRows(analysisRuleQueryColumns()).
		AddRow(
			"rule-queue",
			0.74,
			130,
			"single",
			"queue backlog",
			[]byte(`["scale consumers"]`),
			"pat-queue",
			"rule-queue",
			4,
			"contains",
			"queue backlog",
			[]byte(`[]`),
			[]byte(`{}`),
		)
	store := newAnalysisStore(t, rows)
	result := engine.Analyze(models.Incident{ID: "inc-queue"}, []models.Signal{
		{EventType: "queue", Severity: 4, Message: "queue backlog crossed threshold"},
		{EventType: "health", Severity: 4, Message: "latency elevated"},
	}, store)

	require.Contains(t, strings.Join(result.Hypotheses, " "), "queue backlog")
	require.NotEmpty(t, result.Recommendations)
	require.Equal(t, "rule-based", result.Source)
}

func TestRuleEngine_QueueBacklogRequiresHealthDegradation(t *testing.T) {
	t.Parallel()

	engine := NewRuleEngine()
	rows := sqlmock.NewRows(analysisRuleQueryColumns()).
		AddRow(
			"rule-queue-unmatched",
			0.74,
			130,
			"single",
			"queue backlog",
			[]byte(`["scale consumers"]`),
			"pat-queue-unmatched",
			"rule-queue-unmatched",
			4,
			"contains",
			"cpu spike",
			[]byte(`[]`),
			[]byte(`{}`),
		)
	store := newAnalysisStore(t, rows)
	result := engine.Analyze(models.Incident{ID: "inc-queue-no-health"}, []models.Signal{
		{EventType: "queue", Severity: 5, Message: "queue backlog crossed threshold"},
		{EventType: "health", Severity: 2, Message: "service healthy"},
	}, store)

	require.Equal(t, "hybrid", result.Source)
	require.NotContains(t, strings.Join(result.Hypotheses, " "), "queue")
}

func TestRuleEngine_HybridFallbackWhenNoDeterministicEvidence(t *testing.T) {
	t.Parallel()

	engine := NewRuleEngine()
	rows := sqlmock.NewRows(analysisRuleQueryColumns())
	store := newAnalysisStore(t, rows)
	result := engine.Analyze(models.Incident{ID: "inc-hybrid"}, []models.Signal{{
		EventType: "log",
		Severity:  2,
		Message:   "minor warning",
	}}, store)

	require.Equal(t, "hybrid", result.Source)
	require.Equal(t, []string{"insufficient deterministic evidence"}, result.Hypotheses)
	require.NotEmpty(t, result.Recommendations)
	require.False(t, result.Timestamp.IsZero())
}

func TestRuleEngine_AssignsIncidentIDAndTimestamp(t *testing.T) {
	t.Parallel()

	engine := NewRuleEngine()
	rows := sqlmock.NewRows(analysisRuleQueryColumns()).
		AddRow(
			"rule-meta",
			0.9,
			100,
			"single",
			"database bottleneck",
			[]byte(`["inspect DB connections"]`),
			"pat-meta",
			"rule-meta",
			5,
			"contains",
			"connection refused",
			[]byte(`[]`),
			[]byte(`{}`),
		)
	store := newAnalysisStore(t, rows)
	result := engine.Analyze(models.Incident{ID: "inc-meta"}, []models.Signal{
		{EventType: "database", Severity: 5, Message: "connection refused"},
	}, store)

	require.Equal(t, "inc-meta", result.IncidentID)
	require.WithinDuration(t, time.Now().UTC(), result.Timestamp, 2*time.Second)
}
