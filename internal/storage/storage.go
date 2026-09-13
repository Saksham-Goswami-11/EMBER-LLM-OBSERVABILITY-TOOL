// Package storage is Ember's persistence layer: one embedded SQLite
// database, no external services required. See schema.go for the tables.
package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sakshamgoswami/ember/internal/model"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

// Open creates (if needed) and migrates the SQLite database at path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite allows only one writer at a time; a single connection avoids
	// SQLITE_BUSY errors under concurrent ingestion instead of masking them.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// GenerateAPIKey returns a new random, printable project key.
func GenerateAPIKey() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "ember_" + hex.EncodeToString(buf), nil
}

func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// ValidAPIKeyFormat reports whether key is shaped like a key Ember issues.
// Operator-supplied keys (EMBER_API_KEY) go through this so a typo or an
// accidentally empty environment variable can't become a live credential.
func ValidAPIKeyFormat(key string) bool {
	return len(key) >= 16 && len(key) <= 200 && strings.TrimSpace(key) == key
}

// EnsureProject creates the project if it doesn't exist yet. When key is
// empty a fresh random key is generated; otherwise the operator's key is
// adopted as-is (this is what EMBER_API_KEY sets up). apiKey is only
// non-empty when the project was just created with a generated key —
// callers must print it then, because the plaintext is never stored.
func (s *Store) EnsureProject(ctx context.Context, id, name, key string) (apiKey string, created bool, err error) {
	var existing string
	err = s.db.QueryRowContext(ctx, `SELECT id FROM projects WHERE id = ?`, id).Scan(&existing)
	if err == nil {
		return "", false, nil
	}
	if err != sql.ErrNoRows {
		return "", false, err
	}
	generated := key == ""
	if generated {
		if key, err = GenerateAPIKey(); err != nil {
			return "", false, err
		}
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO projects (id, name, api_key_hash, created_at) VALUES (?, ?, ?, ?)`,
		id, name, hashKey(key), time.Now().UnixMilli())
	if err != nil {
		return "", false, err
	}
	if !generated {
		// The operator already knows this key; don't echo it to the logs.
		return "", true, nil
	}
	return key, true, nil
}

// SetAPIKey replaces the key for an existing project, leaving its traces
// untouched. This is the escape hatch for a key that was never saved from
// the first-run banner — previously the only recovery was deleting the
// database. Returns the key that is now live.
func (s *Store) SetAPIKey(ctx context.Context, projectID, key string) (string, error) {
	if key == "" {
		var err error
		if key, err = GenerateAPIKey(); err != nil {
			return "", err
		}
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE projects SET api_key_hash = ? WHERE id = ?`, hashKey(key), projectID)
	if err != nil {
		return "", err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", fmt.Errorf("no project %q", projectID)
	}
	return key, nil
}

// ValidateAPIKey reports whether key is the current key for projectID.
func (s *Store) ValidateAPIKey(ctx context.Context, projectID, key string) (bool, error) {
	var hash string
	err := s.db.QueryRowContext(ctx, `SELECT api_key_hash FROM projects WHERE id = ?`, projectID).Scan(&hash)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return hash == hashKey(key), nil
}

func (s *Store) ListProjects(ctx context.Context) ([]model.Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, created_at FROM projects ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Project
	for rows.Next() {
		var p model.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.CreatedAtMS); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// TraceBatch is one trace's worth of newly-ingested spans, plus whatever
// session identity the instrumentation attached to them (may be empty).
type TraceBatch struct {
	TraceID   string
	SessionID string
	Spans     []model.Span
}

// WriteTraces persists spans and recomputes each affected trace's rollup
// from the full span set on disk — spans for one trace often arrive across
// more than one export batch, so the aggregate can't trust a single call's
// spans alone.
func (s *Store) WriteTraces(ctx context.Context, projectID string, batches []TraceBatch) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	spanStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO spans (id, trace_id, parent_span_id, name, operation_name, system, model,
			input_tokens, output_tokens, cost_usd, started_at, ended_at, duration_ms, status_code, status_message, attributes)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			parent_span_id=excluded.parent_span_id, name=excluded.name, operation_name=excluded.operation_name,
			system=excluded.system, model=excluded.model, input_tokens=excluded.input_tokens,
			output_tokens=excluded.output_tokens, cost_usd=excluded.cost_usd, started_at=excluded.started_at,
			ended_at=excluded.ended_at, duration_ms=excluded.duration_ms, status_code=excluded.status_code,
			status_message=excluded.status_message, attributes=excluded.attributes
	`)
	if err != nil {
		return err
	}
	defer spanStmt.Close()

	for _, batch := range batches {
		for _, sp := range batch.Spans {
			attrJSON, err := json.Marshal(sp.Attributes)
			if err != nil {
				return fmt.Errorf("marshal attributes: %w", err)
			}
			if _, err := spanStmt.ExecContext(ctx,
				sp.ID, sp.TraceID, nullIfEmpty(sp.ParentSpanID), sp.Name, sp.OperationName, sp.System, sp.Model,
				sp.InputTokens, sp.OutputTokens, sp.CostUSD, sp.StartedAtMS, sp.EndedAtMS, sp.DurationMS,
				sp.StatusCode, sp.StatusMessage, string(attrJSON),
			); err != nil {
				return fmt.Errorf("insert span %s: %w", sp.ID, err)
			}
		}

		if err := s.upsertTraceAggregate(ctx, tx, projectID, batch.TraceID, batch.SessionID); err != nil {
			return err
		}
		if batch.SessionID != "" {
			if err := s.upsertSession(ctx, tx, projectID, batch.SessionID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *Store) upsertTraceAggregate(ctx context.Context, tx *sql.Tx, projectID, traceID, sessionID string) error {
	var startedAt, endedAt, spanCount, errCount, inTok, outTok sql.NullInt64
	var costUSD sql.NullFloat64
	err := tx.QueryRowContext(ctx, `
		SELECT MIN(started_at), MAX(ended_at), COUNT(*),
			SUM(CASE WHEN status_code = 'error' THEN 1 ELSE 0 END),
			SUM(input_tokens), SUM(output_tokens), SUM(cost_usd)
		FROM spans WHERE trace_id = ?`, traceID,
	).Scan(&startedAt, &endedAt, &spanCount, &errCount, &inTok, &outTok, &costUSD)
	if err != nil {
		return fmt.Errorf("aggregate trace %s: %w", traceID, err)
	}

	var rootName string
	_ = tx.QueryRowContext(ctx, `
		SELECT name FROM spans WHERE trace_id = ? AND (parent_span_id IS NULL OR parent_span_id = '')
		ORDER BY started_at ASC LIMIT 1`, traceID).Scan(&rootName)
	if rootName == "" {
		_ = tx.QueryRowContext(ctx, `SELECT name FROM spans WHERE trace_id = ? ORDER BY started_at ASC LIMIT 1`, traceID).Scan(&rootName)
	}

	var repModel string
	_ = tx.QueryRowContext(ctx, `
		SELECT model FROM spans WHERE trace_id = ? AND model != ''
		ORDER BY (input_tokens + output_tokens) DESC LIMIT 1`, traceID).Scan(&repModel)

	status := "ok"
	if errCount.Int64 > 0 {
		status = "error"
	}
	duration := endedAt.Int64 - startedAt.Int64
	if duration < 0 {
		duration = 0
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO traces (id, project_id, session_id, name, started_at, duration_ms, status, span_count, model, input_tokens, output_tokens, cost_usd)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			session_id = COALESCE(NULLIF(excluded.session_id, ''), traces.session_id),
			name = excluded.name, started_at = excluded.started_at, duration_ms = excluded.duration_ms,
			status = excluded.status, span_count = excluded.span_count, model = excluded.model,
			input_tokens = excluded.input_tokens, output_tokens = excluded.output_tokens, cost_usd = excluded.cost_usd
	`, traceID, projectID, sessionID, rootName, startedAt.Int64, duration, status, spanCount.Int64, repModel, inTok.Int64, outTok.Int64, costUSD.Float64)
	return err
}

func (s *Store) upsertSession(ctx context.Context, tx *sql.Tx, projectID, sessionID string) error {
	now := time.Now().UnixMilli()
	_, err := tx.ExecContext(ctx, `
		INSERT INTO sessions (id, project_id, external_user_id, trace_count, first_seen, last_seen)
		VALUES (?, ?, '', 1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			trace_count = (SELECT COUNT(*) FROM traces WHERE session_id = ?),
			last_seen = MAX(sessions.last_seen, excluded.last_seen),
			first_seen = MIN(sessions.first_seen, excluded.first_seen)
	`, sessionID, projectID, now, now, sessionID)
	return err
}

func (s *Store) ListTraces(ctx context.Context, projectID string, limit int, beforeMS int64) ([]model.Trace, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, COALESCE(session_id,''), COALESCE(name,''), started_at, duration_ms, status, span_count,
			COALESCE(model,''), input_tokens, output_tokens, cost_usd
		FROM traces
		WHERE project_id = ? AND (? = 0 OR started_at < ?)
		ORDER BY started_at DESC LIMIT ?`, projectID, beforeMS, beforeMS, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Trace
	for rows.Next() {
		var t model.Trace
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.SessionID, &t.Name, &t.StartedAtMS, &t.DurationMS, &t.Status,
			&t.SpanCount, &t.Model, &t.InputTokens, &t.OutputTokens, &t.CostUSD); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) GetTrace(ctx context.Context, id string) (*model.Trace, []model.Span, error) {
	var t model.Trace
	err := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, COALESCE(session_id,''), COALESCE(name,''), started_at, duration_ms, status, span_count,
			COALESCE(model,''), input_tokens, output_tokens, cost_usd
		FROM traces WHERE id = ?`, id,
	).Scan(&t.ID, &t.ProjectID, &t.SessionID, &t.Name, &t.StartedAtMS, &t.DurationMS, &t.Status,
		&t.SpanCount, &t.Model, &t.InputTokens, &t.OutputTokens, &t.CostUSD)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, trace_id, COALESCE(parent_span_id,''), COALESCE(name,''), COALESCE(operation_name,''),
			COALESCE(system,''), COALESCE(model,''), input_tokens, output_tokens, cost_usd,
			started_at, ended_at, duration_ms, COALESCE(status_code,''), COALESCE(status_message,''), COALESCE(attributes,'{}')
		FROM spans WHERE trace_id = ? ORDER BY started_at ASC`, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var spans []model.Span
	for rows.Next() {
		var sp model.Span
		var attrJSON string
		if err := rows.Scan(&sp.ID, &sp.TraceID, &sp.ParentSpanID, &sp.Name, &sp.OperationName, &sp.System, &sp.Model,
			&sp.InputTokens, &sp.OutputTokens, &sp.CostUSD, &sp.StartedAtMS, &sp.EndedAtMS, &sp.DurationMS,
			&sp.StatusCode, &sp.StatusMessage, &attrJSON); err != nil {
			return nil, nil, err
		}
		_ = json.Unmarshal([]byte(attrJSON), &sp.Attributes)
		spans = append(spans, sp)
	}
	return &t, spans, rows.Err()
}

func (s *Store) ListSessions(ctx context.Context, projectID string, limit int) ([]model.Session, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, COALESCE(external_user_id,''), trace_count, first_seen, last_seen
		FROM sessions WHERE project_id = ? ORDER BY last_seen DESC LIMIT ?`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Session
	for rows.Next() {
		var sess model.Session
		if err := rows.Scan(&sess.ID, &sess.ProjectID, &sess.ExternalUserID, &sess.TraceCount, &sess.FirstSeenMS, &sess.LastSeenMS); err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

func (s *Store) UsageAnalytics(ctx context.Context, projectID string, days int) ([]model.UsageRow, error) {
	if days <= 0 {
		days = 7
	}
	cutoff := time.Now().AddDate(0, 0, -days).UnixMilli()
	rows, err := s.db.QueryContext(ctx, `
		SELECT date(started_at/1000, 'unixepoch') AS day, COALESCE(NULLIF(model,''),'unknown') AS m,
			COUNT(*), SUM(input_tokens), SUM(output_tokens), SUM(cost_usd)
		FROM traces
		WHERE project_id = ? AND started_at >= ?
		GROUP BY day, m
		ORDER BY day ASC`, projectID, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.UsageRow
	for rows.Next() {
		var u model.UsageRow
		if err := rows.Scan(&u.Date, &u.Model, &u.TraceCount, &u.InputTokens, &u.OutputTokens, &u.CostUSD); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
