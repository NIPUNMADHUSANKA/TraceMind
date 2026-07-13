package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"time"
	"tracemind/internal/models"
	"tracemind/internal/util"

	_ "github.com/lib/pq"
)

type PostgresStore struct {
	db *sql.DB
}

const signalDeleteBatchSize = 1000

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(time.Minute)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS signals (
    id text PRIMARY KEY,
    event_type text,
    source text,
    env text,
    timestamp timestamptz,
    severity int,
    message text,
    payload jsonb,
    metadata jsonb
);
`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	_, err = db.Exec(`
CREATE INDEX IF NOT EXISTS signals_timestamp_idx ON signals (timestamp);
`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS incidents (
    id text PRIMARY KEY,
    title text,
    status text,
    severity int,
    impacted_services jsonb,
    environments jsonb,
    signal_ids jsonb,
    analysis_summary text,
    recommendations jsonb,
    created_at timestamptz,
    updated_at timestamptz
);
`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return &PostgresStore{db: db}, nil
}

func (p *PostgresStore) Close() error {
	if p == nil || p.db == nil {
		return nil
	}
	return p.db.Close()
}

func (p *PostgresStore) SaveSignal(sig models.Signal) {
	if sig.ID == "" {
		sig.ID = util.GenID()
	}
	if sig.Timestamp.IsZero() {
		sig.Timestamp = time.Now().UTC()
	}
	sig.Message = SanitizeMessage(sig.Message)
	sig.Payload = RedactPayloadByAllowList(sig.Payload, payloadAllowListSnapshot())

	payloadJSON, err := json.Marshal(sig.Payload)
	if err != nil {
		log.Printf("store: marshal signal payload failed: %v", err)
		return
	}
	metadataJSON, err := json.Marshal(sig.Metadata)
	if err != nil {
		log.Printf("store: marshal signal metadata failed: %v", err)
		return
	}
	_, err = p.db.Exec(`INSERT INTO signals (id, event_type, source, env, timestamp, severity, message, payload, metadata)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT (id) DO UPDATE SET
    event_type = EXCLUDED.event_type,
    source = EXCLUDED.source,
    env = EXCLUDED.env,
    timestamp = EXCLUDED.timestamp,
    severity = EXCLUDED.severity,
    message = EXCLUDED.message,
    payload = EXCLUDED.payload,
    metadata = EXCLUDED.metadata`,
		sig.ID,
		sig.EventType,
		sig.Source,
		sig.Env,
		sig.Timestamp,
		sig.Severity,
		sig.Message,
		payloadJSON,
		metadataJSON)
	if err != nil {
		log.Printf("store: save signal failed: %v", err)
	}
}

func (p *PostgresStore) GetSignal(id string) (models.Signal, bool) {
	row := p.db.QueryRow(`SELECT id, event_type, source, env, timestamp, severity, message, payload, metadata FROM signals WHERE id = $1`, id)
	var sig models.Signal
	var payloadJSON []byte
	var metadataJSON []byte
	if err := row.Scan(&sig.ID, &sig.EventType, &sig.Source, &sig.Env, &sig.Timestamp, &sig.Severity, &sig.Message, &payloadJSON, &metadataJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Signal{}, false
		}
		log.Printf("store: get signal scan failed: %v", err)
		return models.Signal{}, false
	}
	if len(payloadJSON) > 0 {
		if err := json.Unmarshal(payloadJSON, &sig.Payload); err != nil {
			log.Printf("store: unmarshal signal payload failed: %v", err)
			return models.Signal{}, false
		}
	}
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &sig.Metadata); err != nil {
			log.Printf("store: unmarshal signal metadata failed: %v", err)
			return models.Signal{}, false
		}
	}
	return sig, true
}

func (p *PostgresStore) DeleteSignalsOlderThan(cutoff time.Time) int {
	totalDeleted := 0
	for {
		deleted := p.deleteSignalsOlderThanBatch(cutoff, signalDeleteBatchSize)
		totalDeleted += deleted
		if deleted < signalDeleteBatchSize {
			return totalDeleted
		}
	}
}

func (p *PostgresStore) deleteSignalsOlderThanBatch(cutoff time.Time, batchSize int) int {
	res, err := p.db.Exec(`DELETE FROM signals
WHERE ctid IN (
	SELECT ctid FROM signals
	WHERE timestamp < $1
	ORDER BY timestamp ASC
	LIMIT $2
)`, cutoff, batchSize)
	if err != nil {
		log.Printf("store: delete old signals failed: %v", err)
		return 0
	}
	affected, err := res.RowsAffected()
	if err != nil {
		log.Printf("store: rows affected for signal delete failed: %v", err)
		return 0
	}
	return int(affected)
}

func (p *PostgresStore) DeleteIncidentsOlderThan(cutoff time.Time) int {
	totalDeleted := 0
	for {
		deleted := p.DeleteIncidentsOlderThanBatch(cutoff, signalDeleteBatchSize)
		totalDeleted += deleted
		if deleted < signalDeleteBatchSize {
			return totalDeleted
		}
	}
}

func (p *PostgresStore) DeleteIncidentsOlderThanBatch(cutoff time.Time, batchSize int) int {
	res, err := p.db.Exec(`DELETE FROM incidents
	WHERE ctid IN (
		SELECT ctid FROM incidents
		WHERE timestamp < $1
		ORDER BY timestamp ASC
		LIMIT $2
	)`, cutoff, batchSize)
	if err != nil {
		log.Printf("store: delete old incidents failed: %v", err)
		return 0
	}
	affected, err := res.RowsAffected()
	if err != nil {
		log.Printf("store: rows affected for incidents delete failed: %v", err)
		return 0
	}
	return int(affected)
}

func (p *PostgresStore) SaveIncident(inc models.Incident) {
	if inc.ID == "" {
		inc.ID = util.GenID()
	}
	if inc.CreatedAt.IsZero() {
		inc.CreatedAt = time.Now().UTC()
	}
	inc.UpdatedAt = time.Now().UTC()

	signalIDsJSON, err := json.Marshal(inc.SignalIDs)
	if err != nil {
		log.Printf("store: marshal incident signal IDs failed: %v", err)
		return
	}
	impactedJSON, err := json.Marshal(inc.ImpactedServices)
	if err != nil {
		log.Printf("store: marshal incident impacted services failed: %v", err)
		return
	}
	envJSON, err := json.Marshal(inc.Environments)
	if err != nil {
		log.Printf("store: marshal incident environments failed: %v", err)
		return
	}
	recommendationsJSON, err := json.Marshal(inc.Recommendations)
	if err != nil {
		log.Printf("store: marshal incident recommendations failed: %v", err)
		return
	}
	_, err = p.db.Exec(`INSERT INTO incidents (id, title, status, severity, impacted_services, environments, signal_ids, analysis_summary, recommendations, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    status = EXCLUDED.status,
    severity = EXCLUDED.severity,
    impacted_services = EXCLUDED.impacted_services,
    environments = EXCLUDED.environments,
    signal_ids = EXCLUDED.signal_ids,
    analysis_summary = EXCLUDED.analysis_summary,
    recommendations = EXCLUDED.recommendations,
    updated_at = EXCLUDED.updated_at`,
		inc.ID,
		inc.Title,
		inc.Status,
		inc.Severity,
		impactedJSON,
		envJSON,
		signalIDsJSON,
		inc.AnalysisSummary,
		recommendationsJSON,
		inc.CreatedAt,
		inc.UpdatedAt)
	if err != nil {
		log.Printf("store: save incident failed: %v", err)
	}
}

func (p *PostgresStore) UpdateIncidentStatus(id string, status string) error {
	if id == "" {
		return errors.New("store: incident id is required")
	}
	if status == "" {
		return errors.New("store: incident status is required")
	}

	updatedAt := time.Now().UTC()
	res, err := p.db.Exec(`UPDATE incidents SET status = $2, updated_at = $3 WHERE id = $1`, id, status, updatedAt)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (p *PostgresStore) ListIncidents() []models.Incident {
	rows, err := p.db.Query(`SELECT id, title, status, severity, impacted_services, environments, signal_ids, analysis_summary, recommendations, created_at, updated_at FROM incidents`)
	if err != nil {
		log.Printf("store: list incidents query failed: %v", err)
		return nil
	}
	defer rows.Close()

	var incidents []models.Incident
	for rows.Next() {
		var inc models.Incident
		var impactedJSON []byte
		var envJSON []byte
		var signalIDsJSON []byte
		var recommendationsJSON []byte
		if err := rows.Scan(&inc.ID, &inc.Title, &inc.Status, &inc.Severity, &impactedJSON, &envJSON, &signalIDsJSON, &inc.AnalysisSummary, &recommendationsJSON, &inc.CreatedAt, &inc.UpdatedAt); err != nil {
			log.Printf("store: list incidents scan failed: %v", err)
			continue
		}
		if len(impactedJSON) > 0 {
			if err := json.Unmarshal(impactedJSON, &inc.ImpactedServices); err != nil {
				log.Printf("store: unmarshal impacted services failed: %v", err)
				continue
			}
		}
		if len(envJSON) > 0 {
			if err := json.Unmarshal(envJSON, &inc.Environments); err != nil {
				log.Printf("store: unmarshal environments failed: %v", err)
				continue
			}
		}
		if len(signalIDsJSON) > 0 {
			if err := json.Unmarshal(signalIDsJSON, &inc.SignalIDs); err != nil {
				log.Printf("store: unmarshal signal IDs failed: %v", err)
				continue
			}
		}
		if len(recommendationsJSON) > 0 {
			if err := json.Unmarshal(recommendationsJSON, &inc.Recommendations); err != nil {
				log.Printf("store: unmarshal recommendations failed: %v", err)
				continue
			}
		}
		incidents = append(incidents, inc)
	}
	if err := rows.Err(); err != nil {
		log.Printf("store: list incidents rows error: %v", err)
	}
	return incidents
}

func (p *PostgresStore) GetIncident(id string) (models.Incident, bool) {
	row := p.db.QueryRow(`SELECT id, title, status, severity, impacted_services, environments, signal_ids, analysis_summary, recommendations, created_at, updated_at FROM incidents WHERE id = $1`, id)
	var inc models.Incident
	var impactedJSON []byte
	var envJSON []byte
	var signalIDsJSON []byte
	var recommendationsJSON []byte
	if err := row.Scan(&inc.ID, &inc.Title, &inc.Status, &inc.Severity, &impactedJSON, &envJSON, &signalIDsJSON, &inc.AnalysisSummary, &recommendationsJSON, &inc.CreatedAt, &inc.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Incident{}, false
		}
		log.Printf("store: get incident scan failed: %v", err)
		return models.Incident{}, false
	}
	if len(impactedJSON) > 0 {
		if err := json.Unmarshal(impactedJSON, &inc.ImpactedServices); err != nil {
			log.Printf("store: unmarshal incident impacted services failed: %v", err)
			return models.Incident{}, false
		}
	}
	if len(envJSON) > 0 {
		if err := json.Unmarshal(envJSON, &inc.Environments); err != nil {
			log.Printf("store: unmarshal incident environments failed: %v", err)
			return models.Incident{}, false
		}
	}
	if len(signalIDsJSON) > 0 {
		if err := json.Unmarshal(signalIDsJSON, &inc.SignalIDs); err != nil {
			log.Printf("store: unmarshal incident signal IDs failed: %v", err)
			return models.Incident{}, false
		}
	}
	if len(recommendationsJSON) > 0 {
		if err := json.Unmarshal(recommendationsJSON, &inc.Recommendations); err != nil {
			log.Printf("store: unmarshal incident recommendations failed: %v", err)
			return models.Incident{}, false
		}
	}
	return inc, true
}
