import { useMemo, useState } from 'react'
import { api } from '../api'
import { useAsync } from '../useAsync'
import CostChart from '../components/CostChart'
import { formatCost, formatTokens } from '../format'

const RANGES = [7, 30, 90]

export default function Analytics() {
  const [days, setDays] = useState(7)
  const { data: usage, error, loading } = useAsync(() => api.usage('default', days), [days])

  const byDay = useMemo(() => {
    const totals = new Map<string, number>()
    for (const row of usage ?? []) {
      totals.set(row.date, (totals.get(row.date) ?? 0) + row.costUsd)
    }
    return [...totals.entries()]
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([date, value]) => ({ label: date.slice(5), value }))
  }, [usage])

  const byModel = useMemo(() => {
    const totals = new Map<string, { tokens: number; cost: number; traces: number }>()
    for (const row of usage ?? []) {
      const entry = totals.get(row.model) ?? { tokens: 0, cost: 0, traces: 0 }
      entry.tokens += row.inputTokens + row.outputTokens
      entry.cost += row.costUsd
      entry.traces += row.traceCount
      totals.set(row.model, entry)
    }
    return [...totals.entries()].sort((a, b) => b[1].cost - a[1].cost)
  }, [usage])

  const totalCost = byModel.reduce((sum, [, v]) => sum + v.cost, 0)
  const totalTraces = byModel.reduce((sum, [, v]) => sum + v.traces, 0)

  return (
    <div className="page">
      <div className="page-head">
        <h1>Analytics</h1>
        <p className="page-sub">Cost and token usage rolled up by day and model.</p>
      </div>

      <div className="range-toggle">
        {RANGES.map((r) => (
          <button key={r} className={r === days ? 'range-btn active' : 'range-btn'} onClick={() => setDays(r)}>
            {r}d
          </button>
        ))}
      </div>

      {error && <div className="banner banner-error">Couldn't load analytics: {error}</div>}

      {!loading && (!usage || usage.length === 0) ? (
        <div className="empty-state">
          <p className="empty-title">No usage data yet</p>
          <p className="empty-body">Cost and token analytics fill in as traces with model spans arrive.</p>
        </div>
      ) : (
        <>
          <div className="stat-row">
            <div className="stat">
              <div className="stat-value num">{formatCost(totalCost)}</div>
              <div className="stat-label">Total cost, last {days}d</div>
            </div>
            <div className="stat">
              <div className="stat-value num">{totalTraces}</div>
              <div className="stat-label">Traces</div>
            </div>
            <div className="stat">
              <div className="stat-value num">{byModel.length}</div>
              <div className="stat-label">Models used</div>
            </div>
          </div>

          <div className="table-card chart-card">
            <CostChart data={byDay} formatValue={formatCost} />
          </div>

          <div className="table-card">
            <table>
              <thead>
                <tr>
                  <th>Model</th>
                  <th className="num-col">Traces</th>
                  <th className="num-col">Tokens</th>
                  <th className="num-col">Cost</th>
                </tr>
              </thead>
              <tbody>
                {byModel.map(([model, v]) => (
                  <tr key={model}>
                    <td className="mono">{model}</td>
                    <td className="num-col num">{v.traces}</td>
                    <td className="num-col num">{formatTokens(v.tokens)}</td>
                    <td className="num-col num">{formatCost(v.cost)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  )
}
