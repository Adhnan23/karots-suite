/** A tiny trend line for a KPI tile (the mockup's sparkline): the last points,
 * a soft area under them and a dot on today. Decorative — the tile states the
 * figure. */
export function Spark({ values, className }: { values: number[]; className?: string }) {
  if (values.length < 2) return null
  const w = 84, h = 34
  const mx = Math.max(...values), mn = Math.min(...values)
  const x = (i: number) => 2 + ((w - 6) * i) / (values.length - 1)
  const y = (v: number) => 3 + (h - 8) * (1 - (v - mn) / (mx - mn || 1))
  const d = values.map((v, i) => `${i ? 'L' : 'M'}${x(i).toFixed(1)},${y(v).toFixed(1)}`).join('')
  return (
    <svg viewBox={`0 0 ${w} ${h}`} width={w} height={h} aria-hidden className={className}>
      <path d={`${d}L${x(values.length - 1)},${h}L2,${h}Z`} fill="currentColor" opacity={0.12} />
      <path d={d} fill="none" stroke="currentColor" strokeWidth={2} strokeLinejoin="round" />
      <circle cx={x(values.length - 1)} cy={y(values[values.length - 1])} r={3} fill="currentColor" />
    </svg>
  )
}
