import { num } from '@/lib/money'

/** % change from `prev` to `cur`, or null when there's nothing to compare with
 *. Measured against
 * |prev| so a deeper loss reads as a fall. */
export function delta(cur: string, prev: string): number | null {
  const p = num(prev)
  return p === 0 ? null : ((num(cur) - p) / Math.abs(p)) * 100
}

/** `part` as a % of `total`, or null when the total isn't positive. */
export function share(part: string, total: string): number | null {
  const t = num(total)
  return t > 0 ? (num(part) / t) * 100 : null
}

/** One decimal, '—' for nothing; never NaN / Infinity, never "-0.0%". */
export function pctText(p: number | null): string {
  if (p === null || !Number.isFinite(p)) return '—'
  const r = Math.round(p * 10) / 10
  return `${(r === 0 ? 0 : r).toFixed(1)}%`
}

/** "2026-10-01 – 2026-10-07", or the one day. */
export const periodText = (from: string, to: string) => (from === to ? from : `${from} – ${to}`)
