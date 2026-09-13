interface Point {
  label: string
  value: number
}

// A small hand-rolled bar chart — no charting library, consistent with the
// rest of Ember's "no unnecessary dependency" stance. Draws to the real
// data scale: every gridline names a value, and the current day is the one
// emphasized mark, since that's the number that's still changing.
export default function CostChart({ data, formatValue }: { data: Point[]; formatValue: (v: number) => string }) {
  const width = 640
  const height = 220
  const padLeft = 64
  const padBottom = 28
  const padTop = 16
  const plotW = width - padLeft - 12
  const plotH = height - padTop - padBottom

  const max = Math.max(...data.map((d) => d.value), 0.000001)
  const barGap = 8
  const barW = data.length > 0 ? Math.min((plotW - barGap * (data.length - 1)) / data.length, 48) : 0
  const usedW = data.length * barW + Math.max(data.length - 1, 0) * barGap
  const startX = padLeft + (plotW - usedW) / 2

  const gridLines = [0, 0.25, 0.5, 0.75, 1]

  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="cost-chart" role="img" aria-label="Cost by day">
      {gridLines.map((frac) => {
        const y = padTop + plotH * (1 - frac)
        return (
          <g key={frac}>
            <line x1={padLeft} y1={y} x2={width - 8} y2={y} className="chart-grid" />
            <text x={padLeft - 8} y={y + 4} textAnchor="end" className="chart-axis-label num">
              {formatValue(max * frac)}
            </text>
          </g>
        )
      })}
      {data.map((d, i) => {
        const barH = max > 0 ? (d.value / max) * plotH : 0
        const x = startX + i * (barW + barGap)
        const y = padTop + plotH - barH
        const isLast = i === data.length - 1
        return (
          <g key={d.label}>
            <rect
              x={x}
              y={y}
              width={barW}
              height={Math.max(barH, 1)}
              rx={3}
              className={isLast ? 'chart-bar chart-bar-current' : 'chart-bar'}
            />
            <text x={x + barW / 2} y={height - padBottom + 16} textAnchor="middle" className="chart-axis-label">
              {d.label}
            </text>
          </g>
        )
      })}
    </svg>
  )
}
