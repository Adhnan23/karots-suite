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

/** A quantity without forced decimals: 137 → "137", 1.5 → "1.5". */
export const qtyText = (v: unknown) => String(Number(num(v).toFixed(3)))

/** A number as the decimal string the API expects. */
export const dec = (n: number) => r2(n).toFixed(2)
