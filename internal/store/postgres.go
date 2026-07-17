package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"
	"tracemind/internal/models"
	"tracemind/internal/util"

	"github.com/google/uuid"
	"github.com/lib/pq"
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

	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS payload_filter_configs (
	environment text NOT NULL,
	allow_payload text NOT NULL,
	updated_at timestamptz NOT NULL,
	PRIMARY KEY (environment, allow_payload)
);
`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS analysis_rules (
	id text PRIMARY KEY,
	name varchar(200) NOT NULL,
	description text,
	confidence float(20),
	priority int NOT NULL DEFAULT 100,
	enabled boolean NOT NULL DEFAULT TRUE,
	match_type varchar(10) NOT NULL DEFAULT 'ALL',
	hypothesis_template text NOT NULL,
	recommendations jsonb NOT NULL DEFAULT '[]',
	version int NOT NULL DEFAULT 1,
	created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`)

	if err != nil {
		_ = db.Close()
		return nil, err
	}

	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS analysis_rules_patterns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	rule_id text NOT NULL,
    event_type VARCHAR(50),
    source VARCHAR(255),
    environment VARCHAR(100),
    severity_min INTEGER,
    message_match_type VARCHAR(20),
    message_pattern TEXT,
    payload_conditions JSONB NOT NULL DEFAULT '[]'::JSONB,
    variable_mappings JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_signal_patterns_rule
        FOREIGN KEY (rule_id)
        REFERENCES analysis_rules(id)
        ON DELETE CASCADE,

    CONSTRAINT chk_signal_patterns_severity
        CHECK (
            severity_min IS NULL
            OR severity_min BETWEEN 0 AND 100
        ),

    CONSTRAINT chk_signal_patterns_message_match_type
        CHECK (
            message_match_type IS NULL
            OR message_match_type IN (
                'substring',
                'regex',
                'exact'
            )
        ),

    CONSTRAINT chk_signal_patterns_message_config
        CHECK (
            (message_match_type IS NULL AND message_pattern IS NULL)
            OR
            (message_match_type IS NOT NULL AND message_pattern IS NOT NULL)
        ),

    CONSTRAINT chk_signal_patterns_payload_conditions_array
        CHECK (jsonb_typeof(payload_conditions) = 'array'),

    CONSTRAINT chk_signal_patterns_variable_mappings_object
        CHECK (jsonb_typeof(variable_mappings) = 'object')
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
	cutoffLiteral := pq.QuoteLiteral(cutoff.UTC().Format(time.RFC3339Nano))
	query := fmt.Sprintf(`DELETE FROM signals
WHERE ctid IN (
	SELECT ctid FROM signals
	WHERE timestamp < %s::timestamptz
	ORDER BY timestamp ASC
	LIMIT %s
)`, cutoffLiteral, strconv.Itoa(batchSize))

	res, err := p.db.Exec(query)
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
	cutoffLiteral := pq.QuoteLiteral(cutoff.UTC().Format(time.RFC3339Nano))
	query := fmt.Sprintf(`DELETE FROM incidents
	WHERE ctid IN (
		SELECT ctid FROM incidents
		WHERE COALESCE(updated_at, created_at) < %s::timestamptz
		ORDER BY COALESCE(updated_at, created_at) ASC
		LIMIT %s
	)`, cutoffLiteral, strconv.Itoa(batchSize))

	res, err := p.db.Exec(query)
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

func (p *PostgresStore) CreateAnalysisRule(rule models.AnalysisRule) (string, error) {
	if rule.ID == "" {
		rule.ID = util.GenID()
	}
	if rule.Priority == 0 {
		rule.Priority = 100
	}
	if rule.MatchType == "" {
		rule.MatchType = "ALL"
	}
	if rule.Version == 0 {
		rule.Version = 1
	}

	recommendationsJSON, err := json.Marshal(rule.Recommendations)
	if err != nil {
		return "", err
	}

	_, err = p.db.Exec(`INSERT INTO analysis_rules (id, name, description, confidence, priority, enabled, match_type, hypothesis_template, recommendations, version, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
		rule.ID,
		rule.Name,
		rule.Description,
		rule.Confidence,
		rule.Priority,
		rule.Enabled,
		rule.MatchType,
		rule.HypothesisTemplate,
		recommendationsJSON,
		rule.Version,
	)
	if err != nil {
		return "", err
	}

	return rule.ID, nil
}

func (p *PostgresStore) UpdateAnalysisRule(id string, rule models.AnalysisRule) error {
	if id == "" {
		return errors.New("store: analysis rule id is required")
	}

	recommendationsJSON, err := json.Marshal(rule.Recommendations)
	if err != nil {
		return err
	}

	res, err := p.db.Exec(`UPDATE analysis_rules
SET name = $2,
	description = $3,
	confidence = $4,
	priority = $5,
	enabled = $6,
	match_type = $7,
	hypothesis_template = $8,
	recommendations = $9,
	version = $10,
	updated_at = CURRENT_TIMESTAMP
WHERE id = $1`,
		id,
		rule.Name,
		rule.Description,
		rule.Confidence,
		rule.Priority,
		rule.Enabled,
		rule.MatchType,
		rule.HypothesisTemplate,
		recommendationsJSON,
		rule.Version,
	)
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

func (p *PostgresStore) DeleteAnalysisRule(id string) error {
	if id == "" {
		return errors.New("store: analysis rule id is required")
	}

	res, err := p.db.Exec(`DELETE FROM analysis_rules WHERE id = $1`, id)
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

func (p *PostgresStore) CreateAnalysisRulePattern(pattern models.AnalysisRulePattern) (string, error) {
	if pattern.ID == "" {
		pattern.ID = uuid.NewString()
	}

	payloadConditionsJSON, err := json.Marshal(pattern.PayloadConditions)
	if err != nil {
		return "", err
	}
	variableMappingsJSON, err := json.Marshal(pattern.VariableMappings)
	if err != nil {
		return "", err
	}

	_, err = p.db.Exec(`INSERT INTO analysis_rules_patterns (id, rule_id, event_type, source, environment, severity_min, message_match_type, message_pattern, payload_conditions, variable_mappings, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
		pattern.ID,
		pattern.RuleID,
		nullableString(pattern.EventType),
		nullableString(pattern.Source),
		nullableString(pattern.Environment),
		pattern.SeverityMin,
		nullableString(pattern.MessageMatchType),
		nullableString(pattern.MessagePattern),
		payloadConditionsJSON,
		variableMappingsJSON,
	)
	if err != nil {
		return "", err
	}

	return pattern.ID, nil
}

func (p *PostgresStore) UpdateAnalysisRulePattern(id string, pattern models.AnalysisRulePattern) error {
	if id == "" {
		return errors.New("store: analysis rule pattern id is required")
	}

	payloadConditionsJSON, err := json.Marshal(pattern.PayloadConditions)
	if err != nil {
		return err
	}
	variableMappingsJSON, err := json.Marshal(pattern.VariableMappings)
	if err != nil {
		return err
	}

	res, err := p.db.Exec(`UPDATE analysis_rules_patterns
SET rule_id = $2,
	event_type = $3,
	source = $4,
	environment = $5,
	severity_min = $6,
	message_match_type = $7,
	message_pattern = $8,
	payload_conditions = $9,
	variable_mappings = $10,
	updated_at = CURRENT_TIMESTAMP
WHERE id = $1`,
		id,
		pattern.RuleID,
		nullableString(pattern.EventType),
		nullableString(pattern.Source),
		nullableString(pattern.Environment),
		pattern.SeverityMin,
		nullableString(pattern.MessageMatchType),
		nullableString(pattern.MessagePattern),
		payloadConditionsJSON,
		variableMappingsJSON,
	)
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

func (p *PostgresStore) DeleteAnalysisRulePattern(id string) error {
	if id == "" {
		return errors.New("store: analysis rule pattern id is required")
	}

	res, err := p.db.Exec(`DELETE FROM analysis_rules_patterns WHERE id = $1`, id)
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

func nullableString(value interface{}) interface{} {
	if value == "" {
		return nil
	}
	return value
}

func (p *PostgresStore) SavePayloadFilterConfig(environment string, allowList []string) error {
	if environment == "" {
		return errors.New("store: environment is required")
	}

	tx, err := p.db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		_ = tx.Rollback()
	}()

	now := time.Now().UTC()
	for _, payloadKey := range allowList {
		if payloadKey == "" {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO payload_filter_configs (environment, allow_payload, updated_at)
VALUES ($1,$2,$3)`, environment, payloadKey, now); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (p *PostgresStore) GetPayloadFilterConfig(environment string) ([]string, error) {
	if environment == "" {
		return nil, errors.New("store: environment is required")
	}

	rows, err := p.db.Query(`SELECT allow_payload FROM payload_filter_configs WHERE environment = $1 ORDER BY allow_payload ASC`, environment)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	allowList := make([]string, 0, 8)
	for rows.Next() {
		var payloadKey string
		if err := rows.Scan(&payloadKey); err != nil {
			return nil, err
		}
		allowList = append(allowList, payloadKey)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return allowList, nil
}

func deletePayloadFilterConfigRow(tx *sql.Tx, environment string, payloadKey string) (int, error) {
	res, err := tx.Exec(`DELETE FROM payload_filter_configs WHERE environment = $1 AND allow_payload = $2`, environment, payloadKey)
	if err != nil {
		return 0, err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(rows), nil
}

func (p *PostgresStore) DeletePayloadFilterConfig(environment string, payloads []string) (int, error) {
	if environment == "" {
		return 0, errors.New("store: environment is required")
	}
	if len(payloads) == 0 {
		return 0, errors.New("store: payloads must contain at least one key")
	}

	tx, err := p.db.Begin()
	if err != nil {
		return 0, err
	}

	defer func() {
		_ = tx.Rollback()
	}()

	deleted := 0
	for _, payloadKey := range payloads {
		if payloadKey == "" {
			continue
		}
		rows, err := deletePayloadFilterConfigRow(tx, environment, payloadKey)
		if err != nil {
			return 0, err
		}
		deleted += rows
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return deleted, nil
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
