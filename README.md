# TraceMind

Minimal incident ingestion API in Go (Fiber) with queue-backed processing and incident endpoints.

## Quick start

Prerequisites:
- Go 1.25+
- Postgres reachable via `DATABASE_URL`

Run locally (PowerShell):

```powershell
$env:DATABASE_URL = "postgres://postgres:postgres@localhost:5432/tracemind?sslmode=disable"
go run ./cmd/server
```

Server defaults to `http://localhost:8080` unless `PORT` is set.

## Ingest example

```bash
curl -X POST http://localhost:8080/api/ingest \
  -H "Content-Type: application/json" \
  -d '{
    "sourceContext": "local",
    "signals": [
      {
        "eventType": "log",
        "source": "svc",
        "severity": 5,
        "message": "checkout timeout"
      }
    ]
  }'
```

## Useful endpoints

- `GET /api/incidents`
- `GET /api/incidents/:id`
- `GET /api/health/ingestion`