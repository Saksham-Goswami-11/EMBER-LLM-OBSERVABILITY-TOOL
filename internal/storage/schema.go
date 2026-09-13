package storage

const schema = `
CREATE TABLE IF NOT EXISTS projects (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	api_key_hash TEXT NOT NULL,
	created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	external_user_id TEXT,
	trace_count INTEGER NOT NULL DEFAULT 0,
	first_seen INTEGER NOT NULL,
	last_seen INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_project_last_seen ON sessions(project_id, last_seen DESC);

CREATE TABLE IF NOT EXISTS traces (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	session_id TEXT,
	name TEXT,
	started_at INTEGER NOT NULL,
	duration_ms INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'ok',
	span_count INTEGER NOT NULL DEFAULT 0,
	model TEXT,
	input_tokens INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	cost_usd REAL NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_traces_project_started ON traces(project_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_traces_session ON traces(session_id);

CREATE TABLE IF NOT EXISTS spans (
	id TEXT PRIMARY KEY,
	trace_id TEXT NOT NULL,
	parent_span_id TEXT,
	name TEXT,
	operation_name TEXT,
	system TEXT,
	model TEXT,
	input_tokens INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	cost_usd REAL NOT NULL DEFAULT 0,
	started_at INTEGER NOT NULL,
	ended_at INTEGER NOT NULL,
	duration_ms INTEGER NOT NULL DEFAULT 0,
	status_code TEXT,
	status_message TEXT,
	attributes TEXT
);
CREATE INDEX IF NOT EXISTS idx_spans_trace ON spans(trace_id);
`
