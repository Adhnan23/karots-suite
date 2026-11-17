import { ARRANGEMENTS, GAP, MIN, PAD, todayLayout } from '@/features/till/layout/arrangements'

const arrOf = (id: string) => ARRANGEMENTS.find((a) => a.id === id)

/** The width the columns share at screen width w (padding and gaps taken off). */
export function innerOf(id: string, w: number) {
  return w - 2 * PAD - GAP * ((arrOf(id)?.cols.length ?? 3) - 1)
}

/** Each column's narrowest width: a stacked column needs the wider of its two. */
export function minsOf(id: string) {
  return (arrOf(id)?.cols ?? []).map((c) => Math.max(...c.map((a) => MIN[a])))
}

/** Drags bar i (between column i-1 and i) by dx screen px; the two columns beside it
 * trade width and neither goes below its minimum. */
export function dragBar(fr: number[], i: number, dx: number, inner: number, mins: number[]) {
  const lo = mins[i - 1] / inner - fr[i - 1]
  const hi = fr[i] - mins[i] / inner
  const d = Math.min(hi, Math.max(lo, dx / inner))
  return fr.map((f, j) => (j === i - 1 ? f + d : j === i ? f - d : f))
}

/** Where an arrangement starts on a screen w wide: three columns take today's width of
 * each section; stacked ones give a lone products column 55%, a lone cart 50%. Columns
 * below their minimum are raised and the rest shrink to pay for it; null when the three
 * minimums can't fit at all. */
export function startShares(id: string, w: number): number[] | null {
  const a = arrOf(id)
  if (!a) return null
  const inner = innerOf(id, w)
  const mins = minsOf(id).map((m) => m / inner)
  if (mins.reduce((x, y) => x + y) > 1) return null
  let fr: number[]
  if (a.cols.length === 3) {
    const t = todayLayout(Math.max(w, 1200)).fr
    const today = { cat: t[0], cart: t[1], pay: t[2] }
    fr = a.cols.map((c) => today[c[0]])
  } else {
    const lone = a.cols.findIndex((c) => c.length === 1)
    const share = a.cols[lone][0] === 'cat' ? 0.55 : 0.5
    fr = a.cols.map((_, i) => (i === lone ? share : 1 - share))
  }
  const sum = fr.reduce((x, y) => x + y)
  fr = fr.map((f) => f / sum)
  // Raise any column under its minimum, taking the difference from the others' spare width.
  const short = fr.map((f, i) => Math.max(0, mins[i] - f)).reduce((x, y) => x + y)
  if (short > 0) {
    const spare = fr.map((f, i) => Math.max(0, f - mins[i]))
    const total = spare.reduce((x, y) => x + y)
    fr = fr.map((f, i) => (f < mins[i] ? mins[i] : f - (spare[i] / total) * short))
  }
  return fr
}
