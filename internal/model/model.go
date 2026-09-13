// Package model defines Ember's domain types: the shapes that ingestion
// writes and the query API reads back out.
package model

// Span is one gen_ai-annotated unit of work inside a trace: a model call,
// a tool invocation, or a plain nested operation.
type Span struct {
	ID            string            `json:"id"`
	TraceID       string            `json:"traceId"`
	ParentSpanID  string            `json:"parentSpanId,omitempty"`
	Name          string            `json:"name"`
	OperationName string            `json:"operationName,omitempty"` // gen_ai.operation.name: chat, embeddings, execute_tool, ...
	System        string            `json:"system,omitempty"`        // gen_ai.system: openai, anthropic, ...
	Model         string            `json:"model,omitempty"`
	InputTokens   int64             `json:"inputTokens"`
	OutputTokens  int64             `json:"outputTokens"`
	CostUSD       float64           `json:"costUsd"`
	StartedAtMS   int64             `json:"startedAtMs"`
	EndedAtMS     int64             `json:"endedAtMs"`
	DurationMS    int64             `json:"durationMs"`
	StatusCode    string            `json:"statusCode,omitempty"` // "unset" | "ok" | "error"
	StatusMessage string            `json:"statusMessage,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

// Trace is the aggregate view of a request: one root call plus every span
// nested inside it, rolled up into totals the trace list can render fast.
type Trace struct {
	ID           string  `json:"id"`
	ProjectID    string  `json:"projectId"`
	SessionID    string  `json:"sessionId,omitempty"`
	Name         string  `json:"name"`
	StartedAtMS  int64   `json:"startedAtMs"`
	DurationMS   int64   `json:"durationMs"`
	Status       string  `json:"status"` // "ok" | "error"
	SpanCount    int     `json:"spanCount"`
	Model        string  `json:"model,omitempty"`
	InputTokens  int64   `json:"inputTokens"`
	OutputTokens int64   `json:"outputTokens"`
	CostUSD      float64 `json:"costUsd"`
}

// Session groups traces that share a conversation or end-user identity,
// when the instrumentation reports one (session.id / gen_ai.conversation.id).
type Session struct {
	ID             string `json:"id"`
	ProjectID      string `json:"projectId"`
	ExternalUserID string `json:"externalUserId,omitempty"`
	TraceCount     int    `json:"traceCount"`
	FirstSeenMS    int64  `json:"firstSeenMs"`
	LastSeenMS     int64  `json:"lastSeenMs"`
}

// Project scopes ingestion and querying. v1 ships one API key per project
// and no roles — see the PRD's non-goals for why.
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	CreatedAtMS int64  `json:"createdAtMs"`
}

// UsageRow is one (day, model) bucket of the cost/token analytics rollup.
type UsageRow struct {
	Date         string  `json:"date"` // YYYY-MM-DD, UTC
	Model        string  `json:"model"`
	TraceCount   int64   `json:"traceCount"`
	InputTokens  int64   `json:"inputTokens"`
	OutputTokens int64   `json:"outputTokens"`
	CostUSD      float64 `json:"costUsd"`
}
