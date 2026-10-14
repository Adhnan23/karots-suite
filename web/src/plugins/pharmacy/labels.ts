import type { DueStatus, ScriptItem } from './types'

export type Tone = 'danger' | 'warn' | 'muted'

const DAY = 86_400_000
/** Whole local days from a to b (calendar days, not 24-hour blocks). */
function days(a: Date, b: Date) {
  const d = (x: Date) => Date.UTC(x.getFullYear(), x.getMonth(), x.getDate())
  return Math.round((d(b) - d(a)) / DAY)
}

/** A drug's due badge: when it ran / runs out. null before the first hand-over. */
export function dueBadge(status: DueStatus, nextDue: string | null, now = new Date()): { text: string; tone: Tone } | null {
  if (!status || !nextDue) return null
  const due = new Date(nextDue)
  const n = days(now, due)
  if (status === 'overdue') return { text: n === -1 ? 'Ran out yesterday' : n === 0 ? 'Ran out today' : `Ran out ${-n} days ago`, tone: 'danger' }
  if (status === 'soon') return { text: n <= 0 ? 'Runs out today' : n === 1 ? 'Runs out tomorrow' : `Runs out in ${n} days`, tone: 'warn' }
  return { text: `Covered till ${due.getDate()} ${due.toLocaleString('en-GB', { month: 'short' })}`, tone: 'muted' }
}

/** What Add puts in the cart: the tablets still owed (a repeat: a full fill), else the prescribed qty, else 1. */
export function qtyOf(it: Pick<ScriptItem, 'owed' | 'quantity'>): number {
  return Number(it.owed) || Number(it.quantity) || 1
}

export const TONE: Record<Tone, string> = {
  danger: 'text-danger bg-[color-mix(in_srgb,var(--danger)_14%,transparent)]',
  warn: 'text-warning bg-[color-mix(in_srgb,var(--warning)_14%,transparent)]',
  muted: 'text-muted-foreground bg-muted',
}
