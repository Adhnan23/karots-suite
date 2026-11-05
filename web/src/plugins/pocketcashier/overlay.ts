import type { TillCopy } from '@/api/types.gen'
import { dec, num } from '@/lib/money'
import type { Action } from './store/outbox'
import type { CallInput } from './sync'

const CREDIT = /\/till\/customers\/(\d+)\/(payment|refund)$/

const callsIn = (out: Action[]) => out.filter((a) => a.kind === 'call').map((a) => a.body as CallInput)

/** The copy as the phone should show it: what the PC has told it, plus what this
 * phone did that the PC hasn't heard yet. */
export function withOverlay(copy: TillCopy, out: Action[]): TillCopy {
  const moved = new Map<number, number>()
  for (const c of callsIn(out)) {
    const m = CREDIT.exec(c.path)
    if (!m) continue
    const amount = num((c.body as { amount?: string }).amount)
    moved.set(Number(m[1]), (moved.get(Number(m[1])) ?? 0) + (m[2] === 'payment' ? -amount : amount))
  }
  if (!moved.size) return copy
  return { ...copy, customers: copy.customers.map((c) => (moved.has(c.id) ? { ...c, balance: dec(num(c.balance) + moved.get(c.id)!) } : c)) }
}
