const nf = new Intl.NumberFormat('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })

/** Loose number parse for form input / decimal strings; never NaN. */
export function num(v: unknown): number {
  const n = typeof v === 'number' ? v : Number(String(v ?? '').replace(/,/g, '').trim())
  return Number.isFinite(n) ? n : 0
}

/** Round to cents (EPSILON nudge so 1.005 → 1.01, like the server's decimal). */
export const r2 = (n: number) => Math.round((n + Number.EPSILON) * 100) / 100

/** "Rs. 1,234.50" — the one money formatter every screen uses. */
export const money = (sym: string, v: unknown) => `${sym} ${nf.format(r2(num(v)))}`

/** An amount for a tight button: no symbol, no ".00" (1250 → "1,250", 4999.5 → "4,999.50"). */
export const short = (v: unknown) => nf.format(r2(num(v))).replace(/\.00$/, '')

/** A quantity without forced decimals: 137 → "137", 1.5 → "1.5". */
export const qtyText = (v: unknown) => String(Number(num(v).toFixed(3)))

/** A number as the decimal string the API expects. */
export const dec = (n: number) => r2(n).toFixed(2)
