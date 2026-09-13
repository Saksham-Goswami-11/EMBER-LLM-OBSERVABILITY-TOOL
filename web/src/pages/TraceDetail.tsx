import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, type Span } from '../api'
import { useAsync } from '../useAsync'
import StatusPill from '../components/StatusPill'
import Waterfall from '../components/Waterfall'
import { formatCost, formatDuration, formatTime, formatTokens, shortId } from '../format'

export default function TraceDetail() {
  const { id = '' } = useParams()
  const { data, error, loading } = useAsync(() => api.getTrace(id), [id])
  const [selected, setSelected] = useState<Span | null>(null)

  if (loading && !data) return <div className="page">Loading trace…</div>
  if (error) return <div className="page banner banner-error">Couldn't load trace: {error}</div>
  if (!data) return <div className="page">Trace not found.</div>

  const { trace, spans } = data
  const active = selected ?? spans.find((s) => !s.parentSpanId) ?? spans[0] ?? null

  return (
    <div className="page">
      <Link to="/traces" className="back-link">
        ← All traces
      </Link>

      <div className="trace-head">
        <div>
          <h1>{trace.name || '(unnamed trace)'}</h1>
          <p className="page-sub mono">{trace.id}</p>
        </div>
        <StatusPill status={trace.status} />
      </div>

      <div className="stat-row">
        <Stat label="Started" value={formatTime(trace.startedAtMs)} />
        <Stat label="Duration" value={formatDuration(trace.durationMs)} />
        <Stat label="Spans" value={String(trace.spanCount)} />
        <Stat label="Tokens" value={formatTokens(trace.inputTokens + trace.outputTokens)} />
        <Stat label="Cost" value={formatCost(trace.costUsd)} />
        {trace.sessionId && <Stat label="Session" value={shortId(trace.sessionId, 10)} />}
      </div>

      <div className="trace-body">
        <div className="waterfall-card">
          <Waterfall
            spans={spans}
            traceStartMs={trace.startedAtMs}
            traceDurationMs={Math.max(trace.durationMs, 1)}
            selectedId={active?.id}
            onSelect={setSelected}
          />
        </div>

        <aside className="span-panel">
          {active ? <SpanDetails span={active} /> : <p className="dim">Select a span to inspect it.</p>}
        </aside>
      </div>
    </div>
  )
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="stat">
      <div className="stat-value num">{value}</div>
      <div className="stat-label">{label}</div>
    </div>
  )
}

function SpanDetails({ span }: { span: Span }) {
  const attrs = Object.entries(span.attributes ?? {}).filter(([k]) => !k.startsWith('_'))
  return (
    <div>
      <h3 className="span-panel-title">{span.name || '(unnamed span)'}</h3>
      <div className="span-meta">
        {span.system && <span className="pill pill-neutral">{span.system}</span>}
        {span.operationName && <span className="pill pill-accent">{span.operationName}</span>}
        {span.statusCode && <StatusPill status={span.statusCode} />}
      </div>

      <dl className="kv-list">
        <dt>Duration</dt>
        <dd className="num">{formatDuration(span.durationMs)}</dd>
        {span.model && (
          <>
            <dt>Model</dt>
            <dd className="mono">{span.model}</dd>
          </>
        )}
        {(span.inputTokens > 0 || span.outputTokens > 0) && (
          <>
            <dt>Tokens (in / out)</dt>
            <dd className="num">
              {formatTokens(span.inputTokens)} / {formatTokens(span.outputTokens)}
            </dd>
          </>
        )}
        {span.costUsd > 0 && (
          <>
            <dt>Cost</dt>
            <dd className="num">{formatCost(span.costUsd)}</dd>
          </>
        )}
        {span.statusMessage && (
          <>
            <dt>Status message</dt>
            <dd>{span.statusMessage}</dd>
          </>
        )}
      </dl>

      {attrs.length > 0 && (
        <>
          <h4 className="span-panel-subtitle">Attributes</h4>
          <dl className="kv-list kv-list-attrs">
            {attrs.map(([k, v]) => (
              <div key={k} className="kv-row">
                <dt className="mono dim">{k}</dt>
                <dd className="mono">{v}</dd>
              </div>
            ))}
          </dl>
        </>
      )}
    </div>
  )
}
