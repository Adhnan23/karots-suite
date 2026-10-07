import { searchStr } from '@/lib/search'
import { rangeQS } from '@/features/counter/range'

/** A report's period in the URL (spec §6.1: filters live in the URL). Nothing
 * chosen = this month, as the server's reportRange. */
export const rangeDefaults = { preset: 'this-month', from: '', to: '' }

export const rangeSearch = (s: Record<string, unknown>) => ({
  preset: s.preset === undefined ? 'this-month' : searchStr(s.preset), from: searchStr(s.from), to: searchStr(s.to),
})

export type RangeSearch = { preset?: string; from?: string; to?: string }

/** The period as a full Range (defaults filled in). */
export const rangeOf = (s: RangeSearch) => ({ preset: s.preset ?? 'this-month', from: s.from ?? '', to: s.to ?? '' })

/** Query string for a report: the period plus any other non-empty filters. */
export function reportQS(s: RangeSearch & Record<string, string | number | undefined>): string {
  const { preset: _p, from: _f, to: _t, ...rest } = s
  const extra = Object.entries(rest).filter(([, v]) => v !== undefined && v !== '').map(([k, v]) => `&${k}=${encodeURIComponent(String(v))}`).join('')
  return (rangeQS(rangeOf(s)) + extra).replace(/^&/, '')
}
