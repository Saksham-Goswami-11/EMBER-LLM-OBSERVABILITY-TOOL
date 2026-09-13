import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api'
import { useAsync } from '../useAsync'
import StatusPill from '../components/StatusPill'
import { formatCost, formatDuration, formatTime, formatTokens } from '../format'

export default function Traces() {
  const navigate = useNavigate()
  const { data: traces, error, loading, reload } = useAsync(() => api.listTraces(), [])

  useEffect(() => {
    const interval = setInterval(reload, 5000)
    return () => clearInterval(interval)
  }, [reload])

  return (
    <div className="page">
      <div className="page-head">
        <h1>Traces</h1>
        <p className="page-sub">Every request ingested from OTLP, newest first. Refreshes every 5s.</p>
      </div>

      {error && <div className="banner banner-error">Couldn't load traces: {error}</div>}

      {!loading && traces && traces.length === 0 && <EmptyState />}

      {traces && traces.length > 0 && (
        <div className="table-card">
          <table>
            <thead>
              <tr>
                <th>Time</th>
                <th>Name</th>
                <th>Model</th>
                <th>Spans</th>
                <th className="num-col">Duration</th>
                <th className="num-col">Tokens</th>
                <th className="num-col">Cost</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {traces.map((t) => (
                <tr key={t.id} className="clickable-row" onClick={() => navigate(`/traces/${t.id}`)}>
                  <td className="mono dim">{formatTime(t.startedAtMs)}</td>
                  <td>{t.name || <span className="dim">(unnamed)</span>}</td>
                  <td className="mono">{t.model || <span className="dim">—</span>}</td>
                  <td className="num">{t.spanCount}</td>
                  <td className="num-col num">{formatDuration(t.durationMs)}</td>
                  <td className="num-col num">{formatTokens(t.inputTokens + t.outputTokens)}</td>
                  <td className="num-col num">{formatCost(t.costUsd)}</td>
                  <td>
                    <StatusPill status={t.status} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function EmptyState() {
  return (
    <div className="empty-state">
      <p className="empty-title">No traces yet</p>
      <p className="empty-body">
        Point any OpenTelemetry exporter at this instance and traces will show up here within seconds.
      </p>
      <pre className="empty-code">
{`OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://localhost:8080/v1/traces
OTEL_EXPORTER_OTLP_TRACES_HEADERS=Authorization=Bearer <your-api-key>`}
      </pre>
      <p className="empty-body dim">
        The API key was printed to the terminal the first time Ember started. Or run the bundled demo agent
        (<code>examples/demo-agent</code>) to see sample traces immediately.
      </p>
    </div>
  )
}
