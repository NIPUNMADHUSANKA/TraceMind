# Analysis Engine, Durable Queue, and Archive Tiers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the missing MVP capabilities for analysis generation, durable queue lifecycle management, and storage-tier retention behavior.

**Architecture:** The ingest path remains API-first and asynchronous, but queue behavior is upgraded to explicit delivery lifecycle with retries, visibility timeout, and dead-letter handling. Worker correlation hands incidents to a hybrid analysis engine that returns deterministic and evidence-aware outputs. Storage retention adds archive-tier classification and environment-specific defaults to reduce cost while preserving incident evidence.

**Tech Stack:** Go 1.25, Fiber v2, standard library concurrency primitives, PostgreSQL adapter, go test.

## Global Constraints

- Durable queue to decouple API from workers.
- Supports retries, visibility timeouts, and dead-letter handling.
- Raw signal batches are preserved in full for a limited time, suggested as `30 days`.
- Processed event metadata should be retained longer than raw batches, suggested as `365 days`.
- Use storage classes matching access patterns: hot, warm, cold.
- Keep work in the isolated worktree at `.worktrees/traceMind-dev`.

---

### File Map

- Modify: `internal/queue/queue.go` — keep channel compatibility while introducing durable queue facade.
- Create: `internal/queue/reliable_queue.go` — retry, visibility timeout, DLQ behavior.
- Create: `internal/queue/reliable_queue_test.go` — queue lifecycle and failure-path tests.
- Create: `internal/analysis/engine.go` — analyzer interface and orchestration.
- Create: `internal/analysis/rules.go` — deterministic analysis rules.
- Create: `internal/analysis/engine_test.go` — rule and hybrid output tests.
- Modify: `internal/worker/worker.go` — ack/nack queue integration and analysis attachment.
- Modify: `internal/models/models.go` — incident and analysis shape consistency.
- Create: `internal/store/archive_tiers.go` — archive tier and profile logic.
- Create: `internal/store/archive_tiers_test.go` — retention/tiering tests.
- Modify: `internal/store/retention.go` — profile-aware retention scheduling.
- Modify: `internal/api/health.go` — queue retry and DLQ health fields.
- Create: `internal/api/health_queue_metrics_test.go` — health payload assertions.

## Task 1: Durable Queue Delivery Lifecycle

**Files:**
- Modify: `internal/queue/queue.go`
- Create: `internal/queue/reliable_queue.go`
- Test: `internal/queue/reliable_queue_test.go`

**Interfaces:**
- Produces: `type QueueConfig struct { MaxAttempts int; VisibilityTimeout time.Duration }`
- Produces: `type QueueStats struct { Depth int; RetryCount int; DeadLetterCount int; LastProcessedTimestamp time.Time }`
- Produces: `type Delivery struct { Job IngestionJob; Receipt string; Attempt int }`
- Produces: `func NewReliableQueue(cfg QueueConfig) *ReliableQueue`
- Produces: `func (q *ReliableQueue) Enqueue(job IngestionJob) error`
- Produces: `func (q *ReliableQueue) Dequeue(ctx context.Context) (Delivery, error)`
- Produces: `func (q *ReliableQueue) Ack(receipt string) error`
- Produces: `func (q *ReliableQueue) Nack(receipt, reason string) error`

- [ ] Step 1: Write failing tests for retry and dead-letter behavior

```go
func TestReliableQueue_MovesToDLQAfterMaxAttempts(t *testing.T) {
    q := NewReliableQueue(QueueConfig{MaxAttempts: 2, VisibilityTimeout: 20 * time.Millisecond})
    require.NoError(t, q.Enqueue(IngestionJob{IngestionID: "ing-1"}))

    d1, _ := q.Dequeue(context.Background())
    _ = q.Nack(d1.Receipt, "failure-1")
    d2, _ := q.Dequeue(context.Background())
    _ = q.Nack(d2.Receipt, "failure-2")

    stats := q.Stats()
    require.Equal(t, 1, stats.DeadLetterCount)
}
```

- [ ] Step 2: Run test and verify failure

Run: `go test ./internal/queue -run TestReliableQueue_MovesToDLQAfterMaxAttempts -v`
Expected: FAIL because reliable queue and DLQ behavior are not implemented.

- [ ] Step 3: Implement queue lifecycle with in-flight tracking and visibility timeout

```go
func (q *ReliableQueue) Dequeue(ctx context.Context) (Delivery, error) {
    q.mu.Lock()
    defer q.mu.Unlock()
    q.sweepExpired(time.Now().UTC())
    if len(q.ready) == 0 {
        return Delivery{}, ErrQueueEmpty
    }
    item := q.ready[0]
    q.ready = q.ready[1:]
    receipt := uuid.NewString()
    q.inFlight[receipt] = inFlightItem{job: item.job, attempt: item.attempt, visibleAt: time.Now().UTC().Add(q.cfg.VisibilityTimeout)}
    return Delivery{Job: item.job, Receipt: receipt, Attempt: item.attempt}, nil
}
```

- [ ] Step 4: Run queue tests

Run: `go test ./internal/queue -v`
Expected: PASS, including retry and DLQ scenarios.

- [ ] Step 5: Commit

```bash
git add internal/queue
git commit -m "feat(queue): implement retries visibility timeout and dead-letter queue"
```

## Task 2: Worker Integration With Ack/Nack

**Files:**
- Modify: `internal/worker/worker.go`
- Test: `internal/worker/worker_test.go`

**Interfaces:**
- Consumes: `queue.Delivery`
- Produces: queue ack/nack integration for success/failure outcomes.

- [x] Step 1: Write failing worker test for nack path on process failure

```go
func TestWorker_NacksDeliveryOnProcessingError(t *testing.T) {
    // Arrange a delivery that forces process failure and assert Nack called.
}
```

- [x] Step 2: Run worker test and verify failure

Run: `go test ./internal/worker -run TestWorker_NacksDeliveryOnProcessingError -v`
Expected: FAIL until worker uses delivery lifecycle methods.

- [x] Step 3: Implement worker ack/nack flow

```go
delivery, err := q.Dequeue(ctx)
if err != nil {
    return
}
if err := processJob(delivery.Job, st); err != nil {
    _ = q.Nack(delivery.Receipt, err.Error())
    return
}
_ = q.Ack(delivery.Receipt)
```

- [x] Step 4: Run worker and queue tests

Run: `go test ./internal/worker ./internal/queue -v`
Expected: PASS and no incident-processing regression.

- [ ] Step 5: Commit

```bash
git add internal/worker internal/queue
git commit -m "feat(worker): integrate durable queue ack and nack semantics"
```

## Task 3: Archive Tiers and Environment Retention Profiles

**Files:**
- Create: `internal/store/archive_tiers.go`
- Create: `internal/store/archive_tiers_test.go`
- Modify: `internal/store/retention.go`

**Interfaces:**
- Produces: `type ArchiveTier string`
- Produces: `type RetentionProfile struct { RawWindow time.Duration; NormalizedWindow time.Duration; LowSeveritySampling float64 }`
- Produces: `func ResolveArchiveTier(age time.Duration) ArchiveTier`
- Produces: `func RetentionProfileForEnvironment(env string) RetentionProfile`

- [ ] Step 1: Write failing tests for archive tier transitions and profile defaults

```go
func TestRetentionProfileForEnvironment_ProdDefaults(t *testing.T) {
    profile := RetentionProfileForEnvironment("prod")
    require.Equal(t, 30*24*time.Hour, profile.RawWindow)
    require.Equal(t, 365*24*time.Hour, profile.NormalizedWindow)
    require.Equal(t, 0.01, profile.LowSeveritySampling)
}
```

- [ ] Step 2: Run tests and verify failure

Run: `go test ./internal/store -run TestRetentionProfileForEnvironment_ProdDefaults -v`
Expected: FAIL until profile resolver exists.

- [ ] Step 3: Implement tier/profile resolver and hook retention enforcer

```go
func ResolveArchiveTier(age time.Duration) ArchiveTier {
    switch {
    case age <= 7*24*time.Hour:
        return ArchiveTierHot
    case age <= 30*24*time.Hour:
        return ArchiveTierWarm
    default:
        return ArchiveTierCold
    }
}
```

- [ ] Step 4: Run store tests

Run: `go test ./internal/store -v`
Expected: PASS with new tier/profile coverage.

- [ ] Step 5: Commit

```bash
git add internal/store
git commit -m "feat(store): add archive tier resolution and environment retention profiles"
```

## Task 4: Analysis Engine Rule Set

**Files:**
- Create: `internal/analysis/engine.go`
- Create: `internal/analysis/rules.go`
- Create: `internal/analysis/engine_test.go`
- Modify: `internal/models/models.go`

**Interfaces:**
- Produces: `type Analyzer interface { Analyze(incident models.Incident, evidence []models.Signal) models.AnalysisResult }`
- Produces: `func NewRuleEngine() Analyzer`
- Produces: deterministic rules for deployment outage, DB connection failure, queue backlog health degradation.

- [ ] Step 1: Write failing analysis tests for known patterns

```go
func TestRuleEngine_DatabaseFailurePattern(t *testing.T) {
    engine := NewRuleEngine()
    result := engine.Analyze(models.Incident{ID: "inc-db"}, []models.Signal{
        {EventType: "database", Severity: 5, Message: "too many connections"},
        {EventType: "health", Severity: 4, Message: "service timeout"},
    })
    require.Contains(t, strings.Join(result.Hypotheses, " "), "database")
    require.NotEmpty(t, result.Recommendations)
}
```

- [ ] Step 2: Run analysis tests and verify failure

Run: `go test ./internal/analysis -run TestRuleEngine_DatabaseFailurePattern -v`
Expected: FAIL because engine is not implemented.

- [ ] Step 3: Implement rule engine and `source` assignment (`rule-based` or `hybrid`)

```go
if len(hypotheses) == 0 {
    hypotheses = append(hypotheses, "insufficient deterministic evidence")
    source = "hybrid"
}
```

- [ ] Step 4: Run analysis tests

Run: `go test ./internal/analysis -v`
Expected: PASS for all rule scenarios.

- [ ] Step 5: Commit

```bash
git add internal/analysis internal/models/models.go
git commit -m "feat(analysis): add deterministic rules and hybrid fallback output"
```

## Task 5: Worker Analysis Attachment and Incident Persistence

**Files:**
- Modify: `internal/worker/worker.go`
- Test: `internal/worker/worker_test.go`

**Interfaces:**
- Consumes: `analysis.Analyzer`
- Produces: incident records with `analysisSummary`, `recommendations`, and evidence-backed hypothesis output.

- [ ] Step 1: Write failing worker test for analysis field population

```go
func TestProcessJob_AttachesAnalysisToIncident(t *testing.T) {
    // Process a high-severity group and assert AnalysisSummary and Recommendations are set.
}
```

- [ ] Step 2: Run worker test to confirm failure

Run: `go test ./internal/worker -run TestProcessJob_AttachesAnalysisToIncident -v`
Expected: FAIL until worker writes analysis fields.

- [ ] Step 3: Implement analysis attachment in incident upsert flow

```go
analysisResult := analyzer.Analyze(incident, g.Signals)
incident.AnalysisSummary = strings.Join(analysisResult.Hypotheses, "; ")
incident.Recommendations = analysisResult.Recommendations
st.SaveIncident(incident)
```

- [ ] Step 4: Run worker + analysis tests

Run: `go test ./internal/worker ./internal/analysis -v`
Expected: PASS and no regression in existing incident creation/merge tests.

- [ ] Step 5: Commit

```bash
git add internal/worker internal/analysis
git commit -m "feat(worker): persist analyzer output on incident updates"
```

## Task 6: Health API Queue Metrics Extension

**Files:**
- Modify: `internal/api/health.go`
- Create: `internal/api/health_queue_metrics_test.go`

**Interfaces:**
- Consumes: `queue.QueueStats`
- Produces: `/api/health/ingestion` payload fields: `queueDepth`, `retryCount`, `deadLetterCount`, `lastProcessedTimestamp`.

- [ ] Step 1: Write failing API test for queue retry and DLQ metrics

```go
func TestHealthHandler_IncludesQueueLifecycleMetrics(t *testing.T) {
    // Assert retryCount, deadLetterCount, and lastProcessedTimestamp are present.
}
```

- [ ] Step 2: Run API health test to verify failure

Run: `go test ./internal/api -run TestHealthHandler_IncludesQueueLifecycleMetrics -v`
Expected: FAIL until health payload includes lifecycle metrics.

- [ ] Step 3: Implement handler response expansion

```go
"ingestion": fiber.Map{
    "queueDepth": stats.Depth,
    "retryCount": stats.RetryCount,
    "deadLetterCount": stats.DeadLetterCount,
    "lastProcessedTimestamp": stats.LastProcessedTimestamp,
}
```

- [ ] Step 4: Run API and queue tests

Run: `go test ./internal/api ./internal/queue -v`
Expected: PASS with health payload and stats provider coverage.

- [ ] Step 5: Commit

```bash
git add internal/api internal/queue
git commit -m "feat(api): expose queue retry and dead-letter metrics in ingestion health"
```

## Task 7: End-to-End Verification for Queue + Analysis + Archive Profile

**Files:**
- Modify: `test/e2e/ingest_e2e_test.go`
- Create: `test/e2e/queue_analysis_archive_e2e_test.go`

**Interfaces:**
- Consumes: `/api/ingest`, `/api/incidents`, `/api/health/ingestion`
- Produces: E2E assertions covering incident creation, analysis fields, and queue metrics.

- [ ] Step 1: Add failing E2E assertions for analysis summary and queue dead-letter count

```go
require.Eventually(t, func() bool {
    // fetch incidents and ensure analysisSummary is non-empty
    // fetch health and assert deadLetterCount field exists
    return true
}, 5*time.Second, 100*time.Millisecond)
```

- [ ] Step 2: Run E2E tests and confirm failure

Run: `go test ./test/e2e -v`
Expected: FAIL (or SKIP if DATABASE_URL is absent) until analysis and health payload changes are complete.

- [ ] Step 3: Final verification run

Run: `go test ./... -v`
Expected: PASS across all packages.

- [ ] Step 4: Commit

```bash
git add test/e2e
git commit -m "test(e2e): verify durable queue lifecycle, analysis output, and health metrics"
```

---

## Self-Review checklist
- [ ] Spec coverage: tasks map to analysis engine, queue retries/visibility timeout/DLQ, and archive tiers.
- [ ] No placeholders: each step includes concrete code and exact command.
- [ ] Type consistency: queue, worker, and analysis interfaces match across tasks.

## Execution handoff
Plan complete and saved to `docs/superpowers/plans/2026-07-09-analysis-engine-queue-archive.md`.

Two execution options:

1. Subagent-Driven (recommended) - I dispatch a fresh subagent per task, review between tasks, fast iteration
2. Inline Execution - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
