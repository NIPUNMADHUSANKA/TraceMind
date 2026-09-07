# TraceMind

TraceMind is a Go (Fiber) incident ingestion and analysis service.

It accepts batches of operational signals, persists them in Postgres, queues ingestion jobs, correlates incidents in a background worker, and attaches rule-based or hybrid AI-assisted analysis output.

## What it does

- Ingests structured signals via `POST /api/ingest`
- Persists signals and incidents in Postgres
- Processes ingestion jobs through AWS SQS with acknowledgement and visibility-delay retry handling
- Correlates incidents by source + environment within a time window
- Generates analysis output using deterministic rules, with AI fallback when no rules match
- Exposes health and queue lifecycle metrics
- Streams ingestion status updates through Server-Sent Events (SSE)
- Supports payload allow-list configuration per environment
- Supports CRUD APIs for analysis rules and analysis rule patterns

## Tech stack

- Language: Go 1.25
- HTTP server: Fiber v2
- Database: PostgreSQL
- Queue: Amazon SQS (`internal/queue`)
- Optional AI provider: OpenAI (via `OPENAI_API_KEY`)

## Project layout

```
cmd/server/main.go          # App bootstrap and route wiring
internal/api/               # HTTP handlers
internal/worker/            # Background ingestion worker and incident correlation
internal/analysis/          # Rule engine and AI fallback orchestration
internal/store/             # Postgres persistence, retention, payload filtering
internal/queue/             # AWS SQS queue adapter and in-memory test queue
test/e2e/                   # End-to-end test coverage
```

## Architecture

![TraceMind architecture diagram](TraceMind.drawio.svg)

## Prerequisites

- Go 1.25+
- PostgreSQL reachable with a valid DSN
- AWS credentials available through the AWS SDK v2 default credential chain
- An Amazon SQS processing queue

## Quick start (5 minutes)

1. Start Postgres (example with Docker):

```bash
docker run --name tracemind-pg --rm -p 5432:5432 \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=tracemind \
  postgres:16
```

2. Set runtime environment and start API:

```powershell
$env:DATABASE_URL = "postgres://postgres:postgres@localhost:5432/tracemind?sslmode=disable"
$env:APP_ENV = "staging"
$env:AWS_REGION = "ap-southeast-1"
$env:SQS_PROCESSING_QUEUE_URL = "https://sqs.ap-southeast-1.amazonaws.com/123456789012/tracemind-processing"
go run ./cmd/server
```

3. Smoke test the service:

```bash
curl http://localhost:8080/
```

Expected response:

```json
{"status":"ok","message":"TraceMind Fiber app is running"}
```

## Environment variables

- `DATABASE_URL` (required)
  - Example: `postgres://postgres:postgres@localhost:5432/tracemind?sslmode=disable`
- `PORT` (optional)
  - Default: `8080`
- `APP_ENV` (optional)
  - Default: `staging`
  - Used for retention profile and payload allow-list selection
- `AWS_REGION` (required)
  - AWS region used to create the SQS client
- `SQS_PROCESSING_QUEUE_URL` (required)
  - Full URL of the SQS queue used for ingestion jobs
- `OPENAI_API_KEY` (optional)
  - Used only when deterministic rules do not produce hypotheses

Notes:

- The service attempts to load `../../.env` at startup; if missing it continues normally and uses process environment variables.
- `DATABASE_URL` must be present in the process environment when the service starts.

## Database bootstrap behavior

On startup, TraceMind auto-creates required tables/indexes if they do not already exist:

- `signals`
- `incidents`
- `payload_filter_configs`
- `ingestion_statuses`
- `analysis_rules`
- `analysis_rules_patterns`

No separate migration step is required for local development in the current setup.

## Run locally

PowerShell:

```powershell
$env:DATABASE_URL = "postgres://postgres:postgres@localhost:5432/tracemind?sslmode=disable"
$env:APP_ENV = "staging"
$env:AWS_REGION = "ap-southeast-1"
$env:SQS_PROCESSING_QUEUE_URL = "https://sqs.ap-southeast-1.amazonaws.com/123456789012/tracemind-processing"
go run ./cmd/server
```

Server will start on `http://localhost:8080` unless `PORT` is provided.

## Run with Docker

Build image:

```bash
docker build -t tracemind:local .
```

Run container:

```bash
docker run --rm -p 8080:8080 \
  -e DATABASE_URL="postgres://postgres:postgres@host.docker.internal:5432/tracemind?sslmode=disable" \
  -e APP_ENV="staging" \
  -e AWS_REGION="ap-southeast-1" \
  -e SQS_PROCESSING_QUEUE_URL="https://sqs.ap-southeast-1.amazonaws.com/123456789012/tracemind-processing" \
  -e AWS_ACCESS_KEY_ID \
  -e AWS_SECRET_ACCESS_KEY \
  -e AWS_SESSION_TOKEN \
  tracemind:local
```

The AWS credential variables are passed through only when they are needed by your credential setup. The AWS SDK v2 also supports its other default credential sources.

## API overview

### Base routes

- `GET /`
- `POST /api/ingest`
- `GET /api/ingest/:id/events`
- `GET /api/incidents`
- `GET /api/incidents/:id`
- `GET /api/health/ingestion`
- `POST /api/payload-filters/:environment`
- `DELETE /api/payload-filters/:environment`
- `POST /api/analysis-rules`
- `PUT /api/analysis-rules/:id`
- `DELETE /api/analysis-rules/:id`
- `POST /api/analysis-rule-patterns`
- `PUT /api/analysis-rule-patterns/:id`
- `DELETE /api/analysis-rule-patterns/:id`

### Ingest request

`POST /api/ingest`

Payload shape:

```json
{
  "sourceContext": "payment-service-prod",
  "signals": [
    {
      "id": "sig-001",
      "eventType": "log",
      "source": "payment-api",
      "environment": "prod",
      "timestamp": "2026-06-29T10:15:30Z",
      "severity": 3,
      "message": "Payment retry triggered for order 4821",
      "payload": {
        "requestId": "req-123",
        "orderId": "ord-4821",
        "retryCount": 2
      },
      "metadata": {
        "team": "payments",
        "region": "ap-southeast-1"
      }
    }
  ]
}
```

Validation behavior:

- `signals` must contain at least one item
- `eventType` is required and must be one of:
  - `log`, `deployment`, `database`, `queue`, `health`
- `source` is required
- `severity` is required and must be in range `0..5`
- `timestamp` is optional, but if provided it must be RFC3339
- `id` is optional (server generates one when omitted)

Response behavior:

- Returns `200 OK` for mixed accepted/rejected batches
- Returns `400 Bad Request` for malformed payloads or empty signal arrays
- Returns `500 Internal Server Error` if enqueuing fails

Successful response shape:

```json
{
  "ingestionId": "generated-id-or-empty",
  "acceptedCount": 1,
  "rejectedCount": 0,
  "errors": []
}
```

Notes:

- If all signals are rejected, `ingestionId` is empty
- Only accepted signals are enqueued for worker processing
- `sourceContext` is accepted in request payload for context tagging but is not currently used by downstream processing.

### Ingestion status stream

`GET /api/ingest/:id/events`

Streams the lifecycle of an accepted ingestion batch using SSE. The handler subscribes to the in-memory broker first, then reads the persisted status from PostgreSQL. This ordering prevents a live status change from being missed between the status read and subscription, while allowing late subscribers to recover the current state from PostgreSQL.

Each update uses this format:

```text
id: <ingestion-id>
status: pending

```

Status values:

- `pending`: the ingestion status was stored before the batch was enqueued.
- `processing`: the worker started processing the queued batch.
- `completed`: the worker processed the batch successfully.
- `failed`: queueing or worker processing failed.

The stream closes after `completed` or `failed`.

Example:

```bash
curl -N http://localhost:8080/api/ingest/<ingestion-id>/events
```

Response behavior:

- `200 OK` when the ingestion ID exists and the stream is opened.
- `400 Bad Request` when the ingestion ID is missing.
- `404 Not Found` when no ingestion status exists for the ID.
- `500 Internal Server Error` when TraceMind cannot read the status or create the stream.

### Incident query endpoints

- `GET /api/incidents` returns:

```json
{
  "incidents": []
}
```

- `GET /api/incidents/:id`
  - `200 OK` with a single incident when found
  - `404 Not Found` when incident ID does not exist

### Health endpoint

`GET /api/health/ingestion`

Returns queue and incident visibility metrics:

```json
{
  "status": "healthy",
  "available": "5",
  "inFlight": "2",
  "delayed": "0"
}
```

These values are SQS approximate queue counts: available messages, messages not visible while in flight, and delayed messages.

### Payload allow-list configuration

Use these endpoints to control which payload keys are persisted for a given environment.

#### Update allow-list

`POST /api/payload-filters/:environment`

Request body:

```json
{
  "payloads": ["requestId", "service", "region"]
}
```

Example:

```bash
curl -X POST http://localhost:8080/api/payload-filters/staging \
  -H "Content-Type: application/json" \
  -d '{"payloads":["requestId","service","region"]}'
```

#### Delete keys from allow-list

`DELETE /api/payload-filters/:environment`

Request body:

```json
{
  "payloads": ["region"]
}
```

Example:

```bash
curl -X DELETE http://localhost:8080/api/payload-filters/staging \
  -H "Content-Type: application/json" \
  -d '{"payloads":["region"]}'
```

### Analysis rule APIs

Use these endpoints to create and manage deterministic analysis rules and their matching patterns.

#### Create rule

`POST /api/analysis-rules`

Use this endpoint to define a rule that can produce a hypothesis and recommendations when matching signals are found.

Request body:

```json
{
  "name": "Queue Backlog",
  "description": "Detect queue delay patterns",
  "confidence": 0.85,
  "priority": 80,
  "enabled": true,
  "matchType": "single",
  "hypothesisTemplate": "Queue backlog is increasing processing latency",
  "recommendations": ["Scale consumers", "Inspect retry spikes"],
  "version": 1
}
```

Field notes:

- `name` is required and should be a short human-readable rule name.
- `hypothesisTemplate` is required and should describe the likely cause or conclusion when the rule matches.
- `matchType` controls how patterns are evaluated. Use `single` for a rule that matches when any pattern matches, or `correlation` for a rule that requires multiple pattern(all pattern must match) conditions to align.
- `confidence`, `priority`, `enabled`, and `version` are optional but should be set to meaningful values for production use.
- `recommendations` should be an array of actionable follow-up steps.

Example:

```bash
curl -X POST http://localhost:8080/api/analysis-rules \
  -H "Content-Type: application/json" \
  -d '{
    "name":"Queue Backlog",
    "description":"Detect queue delay patterns",
    "confidence":0.85,
    "priority":80,
    "enabled":true,
    "matchType":"single",
    "hypothesisTemplate":"Queue backlog is increasing processing latency",
    "recommendations":["Scale consumers","Inspect retry spikes"],
    "version":1
  }'
```

#### Create rule pattern

`POST /api/analysis-rule-patterns`

Use this endpoint to attach a matching pattern to an existing rule. The pattern tells the engine when a signal should trigger that rule.

Request body:

```json
{
  "ruleId": "<rule-id>",
  "eventType": "queue",
  "source": "worker-service",
  "environment": "staging",
  "severityMin": 3,
  "messageMatchType": "regex",
  "messagePattern": "backlog|timeout",
  "payloadConditions": [],
  "variableMappings": {
    "service": "worker-service"
  }
}
```

Field notes:

- `ruleId` is required and must reference an existing analysis rule ID created by the previous endpoint.
- `eventType` should be a relevant signal type such as `log`, `deployment`, `database`, `queue`, or `health`.
- `source` and `environment` should match the signal origin you want the pattern to target.
- `severityMin` is optional, but if provided it should be an integer threshold for minimum signal severity.
- `messageMatchType` is required when `messagePattern` is supplied, and it must be one of `exact`, `contains`, or `regex`.
- `messagePattern` should contain the literal text or regex expression to match against the signal message.
- `payloadConditions` is optional and can be used to match against payload fields using an object structure like `{ "field": "requestId", "operator": "eq", "value": "123" }`.
- `variableMappings` is optional and can map names used by your rule logic to concrete values.

Example:

```bash
curl -X POST http://localhost:8080/api/analysis-rule-patterns \
  -H "Content-Type: application/json" \
  -d '{
    "ruleId":"<rule-id>",
    "eventType":"queue",
    "source":"worker-service",
    "environment":"staging",
    "severityMin":3,
    "messageMatchType":"regex",
    "messagePattern":"backlog|timeout",
    "payloadConditions":[],
    "variableMappings":{"service":"worker-service"}
  }'
```

Validation notes:

- Rule `name` and `hypothesisTemplate` are required.
- Pattern `ruleId` is required.
- `messageMatchType` and `messagePattern` must be provided together.
- `matchType` on rules should be either `single` or `correlation`.
- `messageMatchType` on patterns should be either `exact`, `contains`, or `regex`.
- Current API surface for rules is write-only CRUD-by-ID (`POST`, `PUT`, `DELETE`); list/get endpoints are not exposed.

## Processing model

1. API validates each incoming signal.
2. Valid signals are persisted to `signals` table.
3. A `pending` ingestion status is persisted and the accepted signals are enqueued as one AWS SQS ingestion job.
4. The worker dequeues the ingestion job, marks the batch `processing`, then groups signals by `source + environment` over a correlation window.
5. High-severity groups can create or update incidents.
6. Analysis engine tries deterministic rules first; if no hypothesis is produced, AI fallback is used.
7. Incident summary and recommendations are stored in `incidents`; the worker publishes `completed` or `failed` for SSE clients.

## Queue behavior

TraceMind uses AWS SQS for durable processing of ingestion jobs. The worker long-polls for one message at a time for up to `20` seconds.

- `Ack` deletes a successfully processed message from SQS.
- `Nack` applies a `30` second visibility delay, after which SQS can deliver the message again.
- SQS provides at-least-once delivery. Worker processing must tolerate duplicate deliveries; message ordering is not guaranteed for a standard SQS queue.
- Configure any maximum receive count and dead-letter queue policy on the SQS queue itself.

## Retention and archive tiers

Retention profile by `APP_ENV`:

- `prod` / `production`: raw signals `30d`, normalized incidents `365d`
- `staging` / `stage`: raw signals `14d`, normalized incidents `90d`
- default (`dev` and others): raw signals `7d`, normalized incidents `30d`

## Testing

Run all tests:

```bash
go test -count=1 ./...
```

Run unit tests only (internal packages):

```bash
go test -count=1 ./internal/...
```

Run e2e tests:

```bash
go test -count=1 ./test/e2e/...
```

## Troubleshooting

- Startup fails with database connection issue:
  - Verify `DATABASE_URL` and Postgres reachability.
- AI recommendations do not appear:
  - Confirm `OPENAI_API_KEY` is set.
  - AI fallback only runs when deterministic rules produce no hypotheses.
- Queue metrics look stuck:
  - Check `GET /api/health/ingestion` and verify worker loop is running.

## License

No license file is currently included in this repository.