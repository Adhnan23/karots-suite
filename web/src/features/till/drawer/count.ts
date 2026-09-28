import type { Denomination, Locker } from '@/api/types.gen'
import { num } from '@/lib/money'

export type Counts = Record<string, string> // denomination value → typed count

export const countTotal = (ds: Denomination[], c: Counts) => ds.reduce((s, d) => s + num(d.value) * num(c[d.value]), 0)

export function toBreakdown(ds: Denomination[], c: Counts): Record<string, number> {
  const b: Record<string, number> = {}
  for (const d of ds) {
    const q = Math.floor(num(c[d.value]))
    if (q > 0) b[d.value] = q
  }
  return b
}

export function fromBreakdown(b: Record<string, number> | undefined): Counts {
  const c: Counts = {}
  for (const [k, v] of Object.entries(b ?? {})) c[k] = String(v)
  return c
}

/** Pre-selected locker for a money-move dialog: the shop default when this cashier
 * can reach it; for cash OUT with untracked cash off, the first locker (never an
 * option the server would reject); else untracked (""). */
export function lockerDefault(lockers: Locker[], defaultId: number, kind: 'in' | 'out', allowUntracked: boolean) {
  if (defaultId && lockers.some((l) => l.id === defaultId)) return String(defaultId)
  if (kind === 'out' && !allowUntracked && lockers.length) return String(lockers[0].id)
  return ''
}

// The opening count lives only in memory: an accidental reload mid-count used to
// lose it. Keep a draft for 12h while the till is still closed.
const DRAFT = 'till.openDraft'

export function loadOpenDraft(now = Date.now()): { counts: Counts; lockerId: string } | null {
  try {
    const d = JSON.parse(localStorage.getItem(DRAFT) ?? 'null')
    if (d && d.c && Object.keys(d.c).length && now - (d.t || 0) < 12 * 3600e3) return { counts: d.c, lockerId: d.l ?? '' }
  } catch {
    /* unreadable draft */
  }
  return null
}

export function saveOpenDraft(counts: Counts, lockerId: string) {
  try {
    localStorage.setItem(DRAFT, JSON.stringify({ c: counts, l: lockerId, t: Date.now() }))
  } catch {
    /* storage unavailable */
  }
}

export function clearOpenDraft() {
  try {
    localStorage.removeItem(DRAFT)
  } catch {
    /* storage unavailable */
  }
}
