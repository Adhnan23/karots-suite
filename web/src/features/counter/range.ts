/** A list's period. preset '' = the latest rows; 'custom' = from/to (inclusive days). */
export type Range = { preset: string; from: string; to: string }
export const LATEST: Range = { preset: '', from: '', to: '' }

const ymd = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`

/** Query string for the server's listRange (reports.ResolveRange presets). Yesterday
 * isn't a server preset, so it goes as an explicit one-day from/to. */
export function rangeQS(r: Range): string {
  if (r.preset === 'yesterday') {
    const y = new Date()
    y.setDate(y.getDate() - 1)
    return `&from=${ymd(y)}&to=${ymd(y)}`
  }
  if (r.preset === 'custom') return r.from || r.to ? `&from=${r.from}&to=${r.to || r.from}` : ''
  return r.preset ? `&preset=${r.preset}` : ''
}
