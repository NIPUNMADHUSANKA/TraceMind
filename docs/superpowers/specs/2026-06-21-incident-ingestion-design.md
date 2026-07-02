# TraceMind Incident Ingestion and Retention Design

## Overview

This design defines how TraceMind captures incoming signals, processes them in a durable worker pipeline, and retains evidence while balancing cost and privacy.

It covers:
- raw signal ingestion and validation
- queue-based worker processing
- incident correlation and analysis
- persistence policy for raw records, normalized metadata, and incident data
- cost and privacy tradeoffs including sampling, redaction, and archive tiers

## Goals

- Capture all valid input signals at the ingestion boundary.
- Decouple ingestion from processing using a durable queue.
- Preserve raw evidence for a limited immutable window.
- Retain normalized metadata long enough for investigation and auditing.
- Minimize storage costs and PII exposure with configurable retention, sampling, and redaction.

## Architecture

### Components

#### Ingest API

- Public REST endpoint: `/api/ingest`
- Validates batch payload and per-signal schema
- Returns `ingestionId`, `acceptedCount`, `rejectedCount`, and validation errors
- Writes validated jobs to the ingestion queue, without performing incident processing inline

#### Ingestion Queue

- Durable queue to decouple API from workers
- Supports retries, visibility timeouts, and dead-letter handling
- Ensures the ingestion API can accept bursts without blocking on work

#### Worker Pipeline

- Dequeues ingestion jobs
- Normalizes each signal into a shared internal event format
- Stores raw signal batches in an immutable archive for a short window
- Enriches signals with context and deduplicates obvious repeats
- Filters noise for incident construction while preserving raw evidence elsewhere
- Correlates related signals into incidents or updates open incidents
- Emits analysis-ready incident records and evidence references

#### Incident Store

- Persists the incident domain model
- Stores incident metadata, timeline, evidence references, analysis, and recommendations
- Keeps incident records long-term for investigation history

#### Analysis Engine

- Combines deterministic rules and AI-assisted reasoning
- Produces root-cause hypotheses, confidence indicators, remediation suggestions, and next steps
- Runs after signals are correlated into incidents

#### Dashboard and Health APIs

- Incident overview and detail pages
- Ingestion health endpoint: `/api/health/ingestion`
- Supports filtering, search, and pipeline visibility

## Data flow

1. Client sends a batch of signals to `/api/ingest`.
2. Ingest API validates the batch and accepted signals.
3. Validated job is written to the ingestion queue.
4. Worker dequeues the job and stores the raw batch in an immutable archive.
5. Worker normalizes signals, deduplicates, enriches, and filters noise for incidents.
6. Worker correlates signals into existing or new incidents.
7. Analysis engine generates incident-level hypotheses and recommendations.
8. Dashboard surfaces incidents, timeline, and ingestion health.

## Persistence policy

### Raw records: short-term immutable archive

- Raw signal batches are preserved in full for a limited time, suggested as `30 days`.
- The raw archive is immutable: once stored, the payload should not be rewritten or deleted except by retention expiration.
- This preserves an audit trail and enables replay if processing logic changes.
- After the retention window, raw batches may be deleted or moved to cheaper cold storage if access is still required.

### Normalized metadata: longer retention

- Processed event metadata should be retained longer than raw batches, suggested as `365 days`.
- Normalized metadata includes:
  - common event fields (`eventType`, `source`, `severity`, `timestamp`)
  - correlation tags
  - deduplicated or enriched classifications
- This data is smaller and more searchable than full raw payloads.
- Longer retention supports incident review, trend analysis, and audits.

### Incident records: business entity retention

- Incident entities should be retained as the primary business record.
- Incident records include:
  - `id`, `title`, `status`, `severity`
  - impacted services, environments, and host context
  - related signal references and timeline
  - analysis summary, hypotheses, and recommendations
- These records can remain long-term, with retention defined by business or compliance needs.

### DLQ and failed jobs

- Dead-lettered or failed ingestion jobs must retain full payload until resolved.
- This preserves debugability for ingestion failures and malformed batches.
- Once the issue is resolved, the DLQ record may be expired.

## Cost and privacy tradeoffs

### Capture-everything forever is not ideal

- Storing all raw input indefinitely is expensive and creates risk.
- Raw signals often include PII or sensitive application payloads.
- The design therefore separates capture from long-term retention.

### Sampling

- Low-severity signals are the largest volume source.
- Suggested default sampling:
  - critical/high severity: 100% retention
  - medium severity: selective retention or adaptive sampling
  - low severity/info: around 1% retention
- Sampling should apply to normalized metadata retention, not to initial raw capture.
- This keeps cost down while preserving essential incident signal.

### Payload redaction

- Redact sensitive fields before storing raw or normalized data.
- Examples:
  - mask user identifiers, email addresses, passwords, auth tokens
  - preserve event context without retaining direct PII
- Redaction protects privacy and compliance while keeping analysis possible.

### Archive tiers

- Use storage classes matching access patterns:
  - hot: recent incident data and metadata for fast queries
  - warm: short-term raw archive and recent normalized events
  - cold: older raw records or archived evidence that is rarely read
- This minimizes cost while preserving access when needed.

## Suggested default retention profile

- Raw signal batch archive: `30 days`, full payload, immutable
- Normalized metadata: `365 days`, with low-severity sampling at `1%`
- Incident records: long-term retention, no sampling
- DLQ / failed jobs: retain until resolved
- API access logs: `90 days`
- Worker debug logs: `7 days`

## Environment configuration

Retention and sampling should be configurable per environment:
- `prod`: longer retention and full SLAs
- `staging`: moderate retention, lower cost
- `dev`: short retention, debug-focused

Example environment defaults:
- `prod`: raw 30 days, normalized 365 days
- `staging`: raw 14 days, normalized 90 days
- `dev`: raw 7 days, normalized 30 days

## Error handling and observability

- Invalid ingest payloads are rejected at the API layer with error details.
- The ingestion queue tracks retries and dead-letter counts.
- Worker processing failures are logged and surfaced in health metrics.
- `/api/health/ingestion` reports queue depth, last processed timestamp, and failed job counts.

## Summary

This design keeps all valid input signals at ingest, but uses worker-side normalization and correlation to decide what becomes incident evidence. It protects debugability and auditability with a short-term raw archive, while controlling ongoing storage cost through longer-lived normalized metadata, sampling, redaction, and archive tiers.
