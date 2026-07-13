# Incident Ingestion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a durable, queue-backed ingestion API and worker pipeline for TraceMind MVP that validates incoming signal batches, persists raw signals for a short window, normalizes signals, and creates incidents for high-severity events.

**Architecture:** A lightweight HTTP API accepts batched signals and enqueues ingestion jobs. A channel-backed queue decouples ingestion from worker processing. Workers normalize and persist signals to an in-memory store (placeholder) or a pluggable persistence layer (Postgres, DynamoDB) and emit incident records. Health and incident APIs surface state.

**Tech Stack:** Go 1.25, Fiber v2, go modules, optional Postgres for persistence, Docker for containerization, Git worktree for isolated development.

## Global Constraints
- Use Go modules (go.mod exists).
- Keep work in the isolated worktree at `.worktrees/traceMind-dev`.
- Retain raw signal batches immutable for 30 days (policy implemented in storage layer).
- Normalized metadata retention: 365 days configurable.

---

### File Map (what to create/modify)
- Create: `cmd/server/main.go` — server entry (already present)
- Create: `internal/models/models.go` — domain types (already present)
- Create: `internal/store/store.go` — pluggable store with in-memory and DB adapters (already present basic)
- Create: `internal/queue/queue.go` — queue abstraction (already present)
- Create: `internal/worker/worker.go` — worker pipeline (already present basic)
- Create: `internal/api/ingest.go` — ingest handler (already present)
- Create: `internal/api/incidents.go` — incidents endpoints (already present)
- Create: `internal/api/health.go` — health endpoint (already present)
- Create: `docs/superpowers/plans/2026-06-21-incident-ingestion.md` — this plan (you are here)
- Tests: `internal/api/main_test.go` (already present)
- Add (optional): `internal/store/postgres.go` — Postgres adapter (new)
- Add: `Dockerfile`, `README.md`, `.github/workflows/ci.yml` (optional)

## Task 1: Verify current baseline (read-only)
**Files:**
- Modify: none

**Interfaces:**
- Consumes: current repository worktree files
- Produces: knowledge of gaps

- [ ] Step 1: Run tests and `go vet` in the worktree

Run:
```bash
cd .worktrees/traceMind-dev
go test ./...
go vet ./...
```
Expected: tests pass (API tests should pass) and `go vet` reports no critical issues.

- [ ] Step 2: Inspect `internal/models/models.go` and `internal/store/store.go` to confirm signatures for `SaveSignal`, `SaveIncident`, `ListIncidents`, `GetIncident`.

Expected: functions with these exact names and signatures exist. If signatures differ, update plan references accordingly.

Commit after verification:
```bash
git add -A
git commit -m "chore: verify baseline for incident ingestion plan"
```

## Task 2: Add persistent storage adapter (Postgres) and configurable in-memory fallback

**Files:**
- Create: `internal/store/postgres.go`
- Modify: `internal/store/store.go` (add interface and factory)
- Test: `internal/store/postgres_test.go`

**Interfaces:**
- Consumes: `models.Signal`, `models.Incident`
- Produces: `store.Store` interface with methods: `SaveSignal(models.Signal)`, `GetSignal(string) (models.Signal, bool)`, `SaveIncident(models.Incident)`, `ListIncidents() []models.Incident`, `GetIncident(string) (models.Incident, bool)`

- [ ] Step 1: Define `store.Storage` interface in `internal/store/store.go` and refactor current concrete `Store` to implement it.

Code (replace at top of `internal/store/store.go`):
```go
package store

import "tracemind/internal/models"

// Storage defines the persistence contract used by workers and API
type Storage interface {
    SaveSignal(models.Signal)
    GetSignal(string) (models.Signal, bool)
    SaveIncident(models.Incident)
    ListIncidents() []models.Incident
    GetIncident(string) (models.Incident, bool)
}
```

- [ ] Step 2: Add Postgres adapter `internal/store/postgres.go` using `database/sql` and `lib/pq` or `pgx`.

Minimal implementation (create file):
```go
package store

import (
    "database/sql"
    "tracemind/internal/models"
    "tracemind/internal/util"
    _ "github.com/lib/pq"
)

type PostgresStore struct { db *sql.DB }

func NewPostgresStore(dsn string) (*PostgresStore, error) {
    db, err := sql.Open("postgres", dsn)
    if err != nil { return nil, err }
    // create tables if not exist (simple schema)
    _, err = db.Exec(`CREATE TABLE IF NOT EXISTS signals (id text PRIMARY KEY, event_type text, source text, env text, timestamp timestamptz, severity int, message text, payload jsonb);`)
    if err != nil { return nil, err }
    _, err = db.Exec(`CREATE TABLE IF NOT EXISTS incidents (id text PRIMARY KEY, title text, status text, severity int, signal_ids jsonb, created_at timestamptz, updated_at timestamptz);`)
    if err != nil { return nil, err }
    return &PostgresStore{db: db}, nil
}

func (p *PostgresStore) SaveSignal(sig models.Signal) { /* implement upsert */ }
func (p *PostgresStore) GetSignal(id string) (models.Signal, bool) { /* implement */ }
func (p *PostgresStore) SaveIncident(inc models.Incident) { /* implement */ }
func (p *PostgresStore) ListIncidents() []models.Incident { /* implement */ }
func (p *PostgresStore) GetIncident(id string) (models.Incident, bool) { /* implement */ }
```

- [ ] Step 3: Write unit tests for Postgres adapter using a local ephemeral DB (Docker) or `sqlmock` in `internal/store/postgres_test.go`.

Run tests:
```bash
go test ./internal/store -v
```
Expected: tests pass.

Commit:
```bash
git add internal/store
git commit -m "feat(store): add Postgres storage adapter"
```

## Task 3: Improve ingestion validation and schema

**Files:**
- Modify: `internal/api/ingest.go`
- Test: `internal/api/ingest_validation_test.go`

**Interfaces:**
- Consumes: JSON body -> `models.IngestRequest`
- Produces: `IngestResponse` with `IngestionId`, `AcceptedCount`, `RejectedCount`, `Errors`

- [ ] Step 1: Add stricter validation rules in `IngestHandler`:
  - `eventType` in allowed set {log,deployment,database,queue,health}
  - `timestamp` optional, parse RFC3339 if present
  - `severity` in 0-5 range

Example code snippet to validate one signal:
```go
valid := map[string]bool{"log":true,"deployment":true,"database":true,"queue":true,"health":true}
if !valid[s.EventType] { rejected++ ; errs = append(errs, "invalid eventType") ; continue }
if s.Severity < 0 || s.Severity > 5 { rejected++ ; errs = append(errs, "invalid severity") ; continue }
```

- [ ] Step 2: Add `internal/api/ingest_validation_test.go` with table-driven tests covering acceptance and rejection.

Run tests:
```bash
go test ./internal/api -run TestIngestValidation -v
```

Commit.

## Task 4: Implement raw signal archive retention and redaction pipeline

**Files:**
- Modify: `internal/store/store.go` or `internal/store/postgres.go`
- Create: `internal/store/retention.go`
- Test: `internal/store/retention_test.go`

**Interfaces:**
- Consumes: saved raw signals
- Produces: retention enforcement background worker or DB TTL policy

- [x] Step 1: Implement a retention enforcer that either:
  - For Postgres: creates a background goroutine that deletes `signals` older than retention window (configurable), or
  - Uses DB native TTL if available (e.g., Timescale or cloud storage lifecycle)

Code sketch (in-memory):
```go
func StartRetentionEnforcer(s Storage, window time.Duration, stop <-chan struct{}) {
  go func(){
    ticker := time.NewTicker(time.Hour)
    for {
      select {
      case <-ticker.C: s.DeleteSignalsOlderThan(time.Now().Add(-window))
      case <-stop: return
      }
    }
  }()
}
```

- [x] Step 2: Redaction: add a utility that removes sensitive keys from `Payload` before storing raw archive. Provide a config-driven allow-list.

Commit.

## Task 5: Worker improvements and correlation strategy

**Files:**
- Modify: `internal/worker/worker.go`
- Test: `internal/worker/worker_test.go`

**Interfaces:**
- Consumes: `queue.IngestionJob` containing signals
- Produces: incidents via `store.Storage.SaveIncident`

- [x] Step 1: Implement correlation window and grouping by `source`/`environment`. Pseudocode:
```go
for job := range q {
  groups := groupBySourceAndWindow(job.Signals, window)
  for _, group := range groups {
    if groupHasHighSeverity(group) { createIncident(group) }
    else mergeIntoExistingIncidentIfRelated(group)
  }
}
```

- [x] Step 2: Add unit tests for grouping and merging logic.

Commit.

## Task 6: End-to-end integration test

**Files:**
- Create: `test/e2e/ingest_e2e_test.go` (or `./internal/e2e` package)

- [ ] Step 1: Start server in test using `exec.Command` or `httptest` style for Fiber
- [ ] Step 2: Post a batch with high-severity signals and assert that an incident appears via `/api/incidents`.

Run:
```bash
go test ./test/e2e -v
```

## Task 7: Dockerfile and README

**Files:**
- Create: `Dockerfile`
- Create: `README.md`

Dockerfile (minimal):
```Dockerfile
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app ./cmd/server

FROM scratch
COPY --from=build /app /app
EXPOSE 3000
CMD ["/app"]
```

README quick start:
```
cd .worktrees/traceMind-dev
go run ./cmd/server
curl -X POST -H 'Content-Type: application/json' http://localhost:3000/api/ingest -d '{"sourceContext":"local","signals":[{"eventType":"log","source":"svc","severity":5}]}'
```

Commit both.

## Task 8: CI

**Files:**
- Create: `.github/workflows/ci.yml`

Minimal CI workflow:
```yaml
name: CI
on: [push, pull_request]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: 1.25
      - name: Run tests
        run: go test ./... -v
```

Commit.

## Task 9: Durable Ingestion Queue (retries, visibility timeout, DLQ)

**Files:**
- Modify: `internal/queue/queue.go`
- Create: `internal/queue/reliable_queue.go`
- Test: `internal/queue/reliable_queue_test.go`
- Modify: `internal/worker/worker.go`

**Interfaces:**
- Consumes: `models.Signal`
- Produces: `type Delivery struct { Job IngestionJob; Receipt string; Attempt int }`
- Produces: `func (q *ReliableQueue) Enqueue(job IngestionJob) error`
- Produces: `func (q *ReliableQueue) Dequeue(ctx context.Context) (Delivery, error)`
- Produces: `func (q *ReliableQueue) Ack(receipt string) error`
- Produces: `func (q *ReliableQueue) Nack(receipt string, reason string) error`
- Produces: `func (q *ReliableQueue) Stats() QueueStats`

- [ ] Step 1: Write failing queue tests for visibility timeout and DLQ threshold

```go
func TestReliableQueue_RequeuesAfterVisibilityTimeout(t *testing.T) {
  q := NewReliableQueue(QueueConfig{MaxAttempts: 3, VisibilityTimeout: 50 * time.Millisecond})
  require.NoError(t, q.Enqueue(IngestionJob{IngestionID: "ing-1"}))

  d1, err := q.Dequeue(context.Background())
  require.NoError(t, err)
  require.Equal(t, 1, d1.Attempt)

  require.Eventually(t, func() bool {
    d2, err := q.Dequeue(context.Background())
    return err == nil && d2.Job.IngestionID == "ing-1" && d2.Attempt == 2
  }, time.Second, 10*time.Millisecond)
}
```

- [ ] Step 2: Run queue test to confirm it fails

Run: `go test ./internal/queue -run TestReliableQueue_RequeuesAfterVisibilityTimeout -v`
Expected: FAIL because reliable queue and visibility timeout behavior are not implemented yet.

- [ ] Step 3: Implement reliable queue and move expired in-flight deliveries back to ready queue

```go
func (q *ReliableQueue) sweepExpired(now time.Time) {
  for receipt, inFlight := range q.inFlight {
    if now.After(inFlight.VisibleAt) {
      delete(q.inFlight, receipt)
      if inFlight.Attempt >= q.cfg.MaxAttempts {
        q.dlq = append(q.dlq, inFlight.job)
        q.failed++
        continue
      }
      q.ready = append(q.ready, queuedItem{job: inFlight.job, attempt: inFlight.Attempt + 1})
    }
  }
}
```

- [ ] Step 4: Update worker to `Ack` successful deliveries and `Nack` failures

```go
delivery, err := rq.Dequeue(ctx)
if err != nil { return }
if err := process(delivery.Job); err != nil {
  _ = rq.Nack(delivery.Receipt, err.Error())
  return
}
_ = rq.Ack(delivery.Receipt)
```

- [ ] Step 5: Run queue and worker tests

Run: `go test ./internal/queue ./internal/worker -v`
Expected: PASS with coverage for retries, visibility timeout, and DLQ handoff behavior.

- [ ] Step 6: Commit

```bash
git add internal/queue internal/worker
git commit -m "feat(queue): add retries visibility timeout and dead-letter handling"
```

## Task 10: Archive Tiers and Retention Profiles

**Files:**
- Create: `internal/store/archive_tiers.go`
- Create: `internal/store/archive_tiers_test.go`
- Modify: `internal/store/retention.go`
- Modify: `internal/store/postgres.go`

**Interfaces:**
- Produces: `type ArchiveTier string`
- Produces: `const (ArchiveTierHot ArchiveTier = "hot"; ArchiveTierWarm ArchiveTier = "warm"; ArchiveTierCold ArchiveTier = "cold")`
- Produces: `func ResolveArchiveTier(age time.Duration) ArchiveTier`
- Produces: `func RetentionProfileForEnvironment(env string) RetentionProfile`

- [ ] Step 1: Write failing tests for tier resolution and environment profiles

```go
func TestResolveArchiveTier(t *testing.T) {
  require.Equal(t, ArchiveTierHot, ResolveArchiveTier(6*time.Hour))
  require.Equal(t, ArchiveTierWarm, ResolveArchiveTier(20*24*time.Hour))
  require.Equal(t, ArchiveTierCold, ResolveArchiveTier(120*24*time.Hour))
}

func TestRetentionProfileForEnvironment(t *testing.T) {
  p := RetentionProfileForEnvironment("prod")
  require.Equal(t, 30*24*time.Hour, p.RawWindow)
  require.Equal(t, 365*24*time.Hour, p.NormalizedWindow)
  require.Equal(t, 0.01, p.LowSeveritySampling)
}
```

- [ ] Step 2: Run tests to verify failure

Run: `go test ./internal/store -run "TestResolveArchiveTier|TestRetentionProfileForEnvironment" -v`
Expected: FAIL until tier/profile code is implemented.

- [ ] Step 3: Implement archive tier and retention profile resolver

```go
func RetentionProfileForEnvironment(env string) RetentionProfile {
  switch env {
  case "prod":
    return RetentionProfile{RawWindow: 30 * 24 * time.Hour, NormalizedWindow: 365 * 24 * time.Hour, LowSeveritySampling: 0.01}
  case "staging":
    return RetentionProfile{RawWindow: 14 * 24 * time.Hour, NormalizedWindow: 90 * 24 * time.Hour, LowSeveritySampling: 0.01}
  default:
    return RetentionProfile{RawWindow: 7 * 24 * time.Hour, NormalizedWindow: 30 * 24 * time.Hour, LowSeveritySampling: 0.01}
  }
}
```

- [ ] Step 4: Wire retention enforcer startup to selected environment profile

```go
profile := RetentionProfileForEnvironment(os.Getenv("APP_ENV"))
store.StartRetentionEnforcer(dbConn, "signals", profile.RawWindow, stopDel)
store.StartRetentionEnforcer(dbConn, "incidents", profile.NormalizedWindow, stopDel)
```

- [ ] Step 5: Run store and server package tests

Run: `go test ./internal/store ./cmd/server -v`
Expected: PASS and no regression on existing retention tests.

- [ ] Step 6: Commit

```bash
git add internal/store cmd/server/main.go
git commit -m "feat(store): add archive tiers and env-specific retention profiles"
```

## Task 11: Analysis Engine (rule-based + hybrid output)

**Files:**
- Create: `internal/analysis/engine.go`
- Create: `internal/analysis/rules.go`
- Create: `internal/analysis/engine_test.go`
- Modify: `internal/models/models.go`
- Modify: `internal/worker/worker.go`

**Interfaces:**
- Produces: `type Analyzer interface { Analyze(models.Incident, []models.Signal) models.AnalysisResult }`
- Produces: `func NewRuleEngine() Analyzer`
- Produces: `func (e *RuleEngine) Analyze(inc models.Incident, evidence []models.Signal) models.AnalysisResult`
- Produces: `func AnalyzeIncidentAndAttach(inc *models.Incident, evidence []models.Signal, analyzer Analyzer)`

- [ ] Step 1: Write failing analysis tests for deployment outage, DB failure, and queue backlog patterns

```go
func TestRuleEngine_DeploymentOutagePattern(t *testing.T) {
  engine := NewRuleEngine()
  inc := models.Incident{ID: "inc-1"}
  evidence := []models.Signal{
    {EventType: "deployment", Severity: 5, Message: "deploy failed"},
    {EventType: "health", Severity: 5, Message: "service unhealthy"},
  }
  out := engine.Analyze(inc, evidence)
  require.Equal(t, "rule-based", out.Source)
  require.NotEmpty(t, out.Hypotheses)
  require.NotEmpty(t, out.Recommendations)
}
```

- [ ] Step 2: Run analysis tests to verify failure first

Run: `go test ./internal/analysis -run TestRuleEngine_DeploymentOutagePattern -v`
Expected: FAIL because analysis engine package and rule logic do not exist yet.

- [ ] Step 3: Implement deterministic rule engine and hybrid source marker when multiple signals contribute

```go
func (e *RuleEngine) Analyze(inc models.Incident, evidence []models.Signal) models.AnalysisResult {
  hypotheses, confidence, actions := evaluateRules(evidence)
  source := "rule-based"
  if len(evidence) > 3 {
    source = "hybrid"
  }
  return models.AnalysisResult{
    IncidentID:      inc.ID,
    Hypotheses:      hypotheses,
    Confidence:      confidence,
    Recommendations: actions,
    Timestamp:       time.Now().UTC(),
    Source:          source,
  }
}
```

- [ ] Step 4: Attach analysis output to incidents from worker flow after correlation

```go
result := analyzer.Analyze(incident, group.Signals)
incident.AnalysisSummary = strings.Join(result.Hypotheses, "; ")
incident.Recommendations = result.Recommendations
store.SaveIncident(incident)
```

- [ ] Step 5: Run analysis and worker tests

Run: `go test ./internal/analysis ./internal/worker -v`
Expected: PASS with analysis fields populated on newly created/updated incidents.

- [ ] Step 6: Commit

```bash
git add internal/analysis internal/models/models.go internal/worker/worker.go
git commit -m "feat(analysis): add hybrid rule engine and incident analysis attachment"
```

## Task 12: Ingestion health metrics for queue retries and dead-letter counts

**Files:**
- Modify: `internal/api/health.go`
- Create: `internal/api/health_queue_metrics_test.go`
- Modify: `cmd/server/main.go`

**Interfaces:**
- Consumes: `queue.QueueStats`
- Produces: `/api/health/ingestion` response fields: `queueDepth`, `retryCount`, `deadLetterCount`, `lastProcessedTimestamp`

- [ ] Step 1: Write failing API health test for retry and DLQ fields

```go
func TestHealthHandler_IncludesRetryAndDLQCounts(t *testing.T) {
  // arrange mocked queue stats and call GET /api/health/ingestion
  // assert payload contains retryCount and deadLetterCount
}
```

- [ ] Step 2: Run the targeted test and confirm failure

Run: `go test ./internal/api -run TestHealthHandler_IncludesRetryAndDLQCounts -v`
Expected: FAIL because response does not yet expose retry/dlq metrics.

- [ ] Step 3: Extend health handler and server wiring to inject queue stats provider

```go
return c.JSON(fiber.Map{
  "ingestion": fiber.Map{
    "queueDepth": qStats.Depth,
    "retryCount": qStats.RetryCount,
    "deadLetterCount": qStats.DeadLetterCount,
    "lastProcessedTimestamp": qStats.LastProcessedTimestamp,
  },
  "incidents": incCount,
})
```

- [ ] Step 4: Run API and queue tests

Run: `go test ./internal/api ./internal/queue -v`
Expected: PASS with health payload including retry and dead-letter metrics.

- [ ] Step 5: Commit

```bash
git add internal/api/health.go internal/api/health_queue_metrics_test.go cmd/server/main.go
git commit -m "feat(api): expose ingestion retry and dead-letter health metrics"
```

---

## Self-Review checklist
- [ ] Spec coverage: ensure ingestion API, queue, worker pipeline, incident entity, health API, retention policies are mapped to tasks above.
- [ ] Spec coverage extension: durable queue retries/visibility timeout/DLQ, archive tiers, and analysis engine outputs are each covered by at least one dedicated task.
- [ ] No placeholders: every Task contains code snippets or exact commands.
- [ ] Type consistency: verify `models` types are used consistently across `internal/*` packages.

## Execution handoff
Plan complete and saved to `docs/superpowers/plans/2026-06-21-incident-ingestion.md`.

Two execution options:

1. Subagent-Driven (recommended) — dispatch a subagent per task and iterate quickly.
2. Inline Execution — I will implement tasks here in this session step-by-step.

Which approach do you prefer? Reply with `subagent` or `inline` (or ask for modifications to the plan).