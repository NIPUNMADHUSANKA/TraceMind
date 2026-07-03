# TraceMind Incident Analysis Design

## Overview

TraceMind is a cloud-hosted incident analysis service. Its primary entity is an Incident, and input signals such as logs, deployment events, database events, queue metrics, and service health events are treated as evidence that can create or enrich incidents.

The MVP focuses on event-driven ingestion, durable processing, incident correlation, hybrid root-cause analysis, and a lightweight dashboard for incident investigation.

## Goals

- Build a cloud-hosted public service with a simple shareable URL.
- Ingest local/raw event batches through a REST API.
- Use an ingestion queue and worker tier for scalable, event-driven processing.
- Persist raw signals, correlated incidents, analysis results, and recommendations.
- Deliver incident-focused investigation guidance rather than traditional log search.
- Keep the dashboard lightweight and secondary to backend incident analysis.

## Scope

### Included in MVP

- Cloud-hosted service with a public dashboard.
- HTTP ingest endpoint for local event batches.
- Durable ingestion queue for processing jobs.
- Worker pipeline for normalizing signals and correlating incidents.
- Incident entity model with status, severity, impacted services, and evidence.
- Hybrid analysis engine with rule-based patterns and AI-assisted reasoning.
- Lightweight dashboard showing incidents, detail pages, ingestion health, and recommendations.
- Persistent storage for raw signals, incidents, analysis outcomes, and audit history.

### Excluded from MVP

- Full-featured log management UI.
- Complex multi-tenant SaaS billing or workspace administration.
- Live streaming ingestion as the first delivery mode.
- Remote observability connectors such as syslog or CloudWatch.

## Architecture

### High-level flow

1. Client posts raw signal batches to the ingest API.
2. API validates input and writes jobs to the ingestion queue.
3. Worker processes queued jobs, normalizes signals, and stores raw event records.
4. Worker correlates signals into incidents or updates existing incidents.
5. Analysis engine produces root-cause hypotheses and actionable recommendations.
6. Dashboard surfaces incident summaries, timelines, analysis status, and ingestion health.

### Components

#### Ingest API

- Public REST endpoint: `/api/ingest`
- Accepts a batch of structured signals
- Validates required fields and schema
- Enqueues ingestion jobs after validation
- Returns structured success/failure responses

#### Ingestion Queue

- Durable queue mechanism for resiliency and scalability
- Decouples ingest API from processing
- Supports retries and dead-letter handling for failed jobs

#### Worker Pipeline

- Dequeues ingestion jobs
- Normalizes each signal into a shared event format
- Stores raw signals and associated metadata
- Correlates signals into incidents based on temporal and contextual relationships
- Updates incident state and attaches supporting evidence

#### Incident Store

- Persisted domain entity for alarms and failures
- Incident fields:
  - `id`, `title`, `status`, `severity`, `priority`
  - affected service(s), environment, host
  - incident timeline
  - correlated signal references
  - analysis summary and recommendations
  - creation and update timestamps

#### Analysis Engine

- Hybrid model combining:
  - deterministic rules for common failure patterns
  - AI-assisted reasoning for open-ended incident diagnosis
- Produces:
  - ranked root-cause hypotheses
  - confidence indicators
  - remediation suggestions
  - investigative next steps

#### Dashboard

- Lightweight UI that supports:
  - incident list with status and recency
  - incident detail view with timeline and evidence
  - analysis summary and recommendation panel
  - ingestion health/status overview
- Dashboard remains secondary to incident processing and analysis.

## Data model

### Signal record

- `id` (generated)
- `eventType`: one of `log`, `deployment`, `database`, `queue`, `health`
- `source`: service or host identifier
- `environment`: e.g. `prod`, `staging`
- `timestamp`
- `severity`
- `message` / `payload`
- `metadata` (optional key/value map)

### Incident entity

- `id`
- `title`
- `status`: e.g. `new`, `investigating`, `resolved`
- `severity`
- `impactedServices`
- `environments`
- `createdAt`, `updatedAt`
- `signalIds`
- `analysisSummary`
- `recommendations`
- `hypotheses`
- `evidenceNotes`

### Analysis result

- `incidentId`
- `rootCauseHypotheses`
- `confidenceScores`
- `recommendedActions`
- `analysisTimestamp`
- `source`: `rule-based`, `ai-assisted`, or `hybrid`

## Incident processing

### Correlation strategy

- Group signals that share service/host/context and occur in the same incident window.
- Use signal type and severity to prioritize incident creation.
- Merge new signals into existing open incidents when they appear related.
- Start a new incident for signals that represent distinct failures.

### Analysis strategy

- Apply deterministic rules first for known patterns such as:
  - deployment-induced service outages
  - database connection failures
  - queue backlog spikes affecting service health
- Use AI-assisted reasoning to generate:
  - incident summaries
  - root cause hypotheses for novel or ambiguous failures
  - investigation guidance and remediation suggestions

## API design

### POST /api/ingest

Request body:

- `sourceContext`
- `signals`: array of signal records

Response:

- `ingestionId`
- `acceptedCount`
- `rejectedCount`
- list of validation errors if any

### GET /api/incidents

- supports filtering by status, service, severity, and time range
- supports pagination

### GET /api/incidents/{id}

- includes incident summary, timeline, evidence, analysis, and recommendations

### GET /api/health/ingestion

- returns queue depth, last processed timestamp, and failed job counts

## Dashboard MVP

- Incident overview page
- Incident detail page with timeline and analysis
- Ingestion status panel with pipeline health and recent ingestion activity
- Search/filter on incidents by status, service, and severity

## Deployment and hosting

- Cloud-hosted service from the outset
- Public-facing URL for dashboard and API access
- Architecture supports future SaaS extension
- Initial deployment can use platform-managed cloud services

## Non-goals for MVP

- Full multi-tenant workspace and billing support
- Deep observability connector catalog
- Real-time streaming event ingestion as first delivery mode
- Complete log search and indexing UX

## Future roadmap

- Add streaming ingestion and event subscriptions
- Add richer SaaS multi-tenant workspace model
- Add alerting and collaboration tools
- Add advanced incident lineage and root-cause traceability
- Add connector integrations for external observability sources

## Testing strategy

- Unit tests for ingest validation, signal normalization, and incident correlation
- Integration tests for API -> queue -> worker -> incident lifecycle
- Dashboard UI tests for incident list and incident detail rendering
- End-to-end tests for ingesting batch signals and verifying incident creation
