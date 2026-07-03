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

---

## Self-Review checklist
- [ ] Spec coverage: ensure ingestion API, queue, worker pipeline, incident entity, health API, retention policies are mapped to tasks above.
- [ ] No placeholders: every Task contains code snippets or exact commands.
- [ ] Type consistency: verify `models` types are used consistently across `internal/*` packages.

## Execution handoff
Plan complete and saved to `docs/superpowers/plans/2026-06-21-incident-ingestion.md`.

Two execution options:

1. Subagent-Driven (recommended) — dispatch a subagent per task and iterate quickly.
2. Inline Execution — I will implement tasks here in this session step-by-step.

Which approach do you prefer? Reply with `subagent` or `inline` (or ask for modifications to the plan).