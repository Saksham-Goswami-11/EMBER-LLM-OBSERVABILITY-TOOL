import type { Span } from '../api'
import { formatDuration } from '../format'

interface Row {
  span: Span
  depth: number
}

// Turns the flat span list OTLP gives us into an ordered, indented row
// list: parent-first, children sorted by start time under their parent.
// Spans whose reported parent never arrived (a partial export) fall back
// to being roots, so nothing silently disappears from the view.
function buildRows(spans: Span[]): Row[] {
  const byId = new Map(spans.map((s) => [s.id, s]))
  const childrenOf = new Map<string, Span[]>()
  const roots: Span[] = []

  for (const s of spans) {
    const parentId = s.parentSpanId && byId.has(s.parentSpanId) ? s.parentSpanId : ''
    if (!parentId) {
      roots.push(s)
      continue
    }
    if (!childrenOf.has(parentId)) childrenOf.set(parentId, [])
    childrenOf.get(parentId)!.push(s)
  }
  for (const list of childrenOf.values()) list.sort((a, b) => a.startedAtMs - b.startedAtMs)
  roots.sort((a, b) => a.startedAtMs - b.startedAtMs)

  const rows: Row[] = []
  const visit = (span: Span, depth: number) => {
    rows.push({ span, depth })
    for (const child of childrenOf.get(span.id) ?? []) visit(child, depth + 1)
  }
  for (const root of roots) visit(root, 0)
  return rows
}

function barKind(span: Span): 'model' | 'tool' | 'other' {
  if (span.operationName === 'execute_tool') return 'tool'
  if (span.operationName) return 'model'
  return 'other'
}

export default function Waterfall({
  spans,
  traceStartMs,
  traceDurationMs,
  selectedId,
  onSelect,
}: {
  spans: Span[]
  traceStartMs: number
  traceDurationMs: number
  selectedId?: string
  onSelect: (span: Span) => void
}) {
  const rows = buildRows(spans)
  const total = Math.max(traceDurationMs, 1)

  return (
    <div className="waterfall" role="table" aria-label="Trace span waterfall">
      {rows.map(({ span, depth }) => {
        const leftPct = Math.min(Math.max(((span.startedAtMs - traceStartMs) / total) * 100, 0), 100)
        const widthPct = Math.min(Math.max((span.durationMs / total) * 100, 0.6), 100 - leftPct)
        const kind = barKind(span)
        const isError = span.statusCode === 'error'
        const isSelected = span.id === selectedId
        return (
          <button
            key={span.id}
            type="button"
            role="row"
            className={`wf-row${isSelected ? ' wf-row-selected' : ''}`}
            onClick={() => onSelect(span)}
          >
            <span className="wf-label" style={{ paddingLeft: 10 + depth * 16 }} title={span.name}>
              {depth > 0 && <span className="wf-branch" aria-hidden="true" />}
              <span className="wf-name">{span.name || '(unnamed span)'}</span>
              {span.model && <span className="wf-model">{span.model}</span>}
            </span>
            <span className="wf-track">
              <span
                className={`wf-bar wf-bar-${kind}${isError ? ' wf-bar-error' : ''}`}
                style={{ left: `${leftPct}%`, width: `${widthPct}%` }}
              />
            </span>
            <span className="wf-duration num">{formatDuration(span.durationMs)}</span>
          </button>
        )
      })}
    </div>
  )
}
