import type { SellLayout } from '@/api/types.gen'

/** The Sell page's three grid areas (TillScreen). */
export type Area = 'cat' | 'cart' | 'pay'

/** Each arrangement's columns, left to right; a column of two stacks them top to
 * bottom. The server keeps the same ids (internal/features/settings/sell_layouts.go). */
export const ARRANGEMENTS: { id: string; cols: Area[][]; label: string }[] = [
  { id: 'PCY', cols: [['cat'], ['cart'], ['pay']], label: 'Today' },
  { id: 'CPY', cols: [['cart'], ['cat'], ['pay']], label: 'Cart first' },
  { id: 'PYC', cols: [['cat'], ['pay'], ['cart']], label: 'Pay middle' },
  { id: 'CYP', cols: [['cart'], ['pay'], ['cat']], label: 'Products right' },
  { id: 'YCP', cols: [['pay'], ['cart'], ['cat']], label: 'Mirror' },
  { id: 'P_CY', cols: [['cat'], ['cart', 'pay']], label: 'Cart over pay' },
  { id: 'CY_P', cols: [['cart', 'pay'], ['cat']], label: '…on the left' },
  { id: 'PY_C', cols: [['cat', 'pay'], ['cart']], label: 'Cart full height' },
  { id: 'C_PY', cols: [['cart'], ['cat', 'pay']], label: '…on the left' },
  { id: 'P_YC', cols: [['cat'], ['pay', 'cart']], label: 'Pay over cart' },
]

/** Each section's narrowest width in px (the server refuses less). */
export const MIN: Record<Area, number> = { cat: 300, cart: 340, pay: 330 }
/** The page's padding on each side, and the gap between sections. */
export const PAD = 14
export const GAP = 14

export const sizeKey = (w: number, h: number) => `${Math.round(w)}x${Math.round(h)}`

const n = (x: number) => String(+x.toFixed(4))
const clamp = (lo: number, v: number, hi: number) => Math.min(hi, Math.max(lo, v))

/** Of keys "<w>x<h>", the one nearest this window whose width and height are each
 * within 10%; null when none is that close. */
export function nearestKey(keys: string[], w: number, h: number): string | null {
  let best: string | null = null
  let bestD = Infinity
  for (const key of keys) {
    const m = /^(\d+)x(\d+)$/.exec(key)
    if (!m) continue
    const dw = Math.abs(w - +m[1]) / +m[1]
    const dh = Math.abs(h - +m[2]) / +m[2]
    if (dw > 0.1 || dh > 0.1 || dw + dh >= bestD) continue
    best = key
    bestD = dw + dh
  }
  return best
}

/** The saved layout for the size nearest this window (see nearestKey); null = today's layout. */
export function pickLayout(layouts: Record<string, SellLayout> | undefined, w: number, h: number): SellLayout | null {
  const key = nearestKey(Object.keys(layouts ?? {}), w, h)
  return key ? layouts![key] : null
}

/** CSS grid for a layout; null when the arrangement is unknown or the shares don't
 * fit it (an old or broken save shows today's layout). */
export function gridOf(l: SellLayout) {
  const a = ARRANGEMENTS.find((x) => x.id === l.arr)
  if (!a || l.fr.length !== a.cols.length || l.fr.some((f) => !(f > 0))) return null
  const stacked = a.cols.some((c) => c.length === 2)
  const split = clamp(0.2, l.split || 0.5, 0.8)
  const row = (i: number) => a.cols.map((c) => c[Math.min(i, c.length - 1)]).join(' ')
  return {
    gridTemplateColumns: l.fr.map((f) => `minmax(0,${n(f)}fr)`).join(' '),
    gridTemplateRows: stacked ? `minmax(0,${n(split)}fr) minmax(0,${n(1 - split)}fr)` : 'minmax(0,1fr)',
    gridTemplateAreas: stacked ? `'${row(0)}' '${row(1)}'` : `'${row(0)}'`,
  }
}

/** Today's layout at width w, as shares (what Appearance starts from): three columns
 * from 1200 px, the cart and pay at their clamped widths; below that cart over pay, 2 to 3. */
export function todayLayout(w: number): SellLayout {
  if (w >= 1200) {
    const inner = w - 2 * PAD - 2 * GAP
    const cart = clamp(392, w * 0.25, 480)
    const pay = clamp(356, w * 0.23, 440)
    return { arr: 'PCY', fr: [(inner - cart - pay) / inner, cart / inner, pay / inner], split: 0, card: 'L', text: 100 }
  }
  const inner = w - 2 * PAD - GAP
  const right = clamp(360, w * 0.4, 460)
  return { arr: 'P_CY', fr: [(inner - right) / inner, right / inner], split: 0.4, card: 'L', text: 100 }
}
