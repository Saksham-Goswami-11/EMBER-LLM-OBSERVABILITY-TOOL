// Thin fetch wrappers over Ember's query API. No client library needed —
// it's the same REST API the OTel-facing docs point curl at.

export interface Trace {
  id: string
  projectId: string
  sessionId?: string
  name: string
  startedAtMs: number
  durationMs: number
  status: 'ok' | 'error'
  spanCount: number
  model?: string
  inputTokens: number
  outputTokens: number
  costUsd: number
}

export interface Span {
  id: string
  traceId: string
  parentSpanId?: string
  name: string
  operationName?: string
  system?: string
  model?: string
  inputTokens: number
  outputTokens: number
  costUsd: number
  startedAtMs: number
  endedAtMs: number
  durationMs: number
  statusCode?: string
  statusMessage?: string
  attributes?: Record<string, string>
}

export interface TraceDetail {
  trace: Trace
  spans: Span[]
}

export interface Session {
  id: string
  projectId: string
  externalUserId?: string
  traceCount: number
  firstSeenMs: number
  lastSeenMs: number
}

export interface UsageRow {
  date: string
  model: string
  traceCount: number
  inputTokens: number
  outputTokens: number
  costUsd: number
}

async function get<T>(url: string): Promise<T> {
  const res = await fetch(url)
  if (!res.ok) {
    const body = await res.text().catch(() => '')
    throw new Error(`${res.status} ${res.statusText}${body ? `: ${body}` : ''}`)
  }
  return (await res.json()) as T
}

export const api = {
  listTraces: (project = 'default', limit = 200) =>
    get<Trace[]>(`/api/traces?project=${encodeURIComponent(project)}&limit=${limit}`),
  getTrace: (id: string) => get<TraceDetail>(`/api/traces/${encodeURIComponent(id)}`),
  listSessions: (project = 'default') => get<Session[]>(`/api/sessions?project=${encodeURIComponent(project)}`),
  usage: (project = 'default', days = 7) =>
    get<UsageRow[]>(`/api/analytics/usage?project=${encodeURIComponent(project)}&days=${days}`),
}
