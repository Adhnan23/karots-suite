import type { CopyHeld, TillCopy } from '@/api/types.gen'
import { dec, num } from '@/lib/money'
import type { Action } from './store/outbox'

type Pay = { method: string; amount: string }

/** Cash places a till may take a float from or bank into (cards and banks hold no notes). */
export const cashPlaces = (copy: TillCopy) => copy.places.filter((p) => p.cashierAccess && p.kind !== 'bank' && p.kind !== 'card')

const mine = (out: Action[], userId: number) => out.filter((a) => a.userId === userId)

/** Is this person's drawer open: the copy's sessions, then what waits to be sent. */
export function drawerOpen(copy: TillCopy, userId: number, out: Action[]): boolean {
  let open = copy.sessions.some((s) => s.userId === userId)
  for (const a of mine(out, userId)) {
    if (a.kind === 'open') open = true
    if (a.kind === 'close') open = false
  }
  return open
}

/** The cash the drawer should hold: the copy's figure plus what the PC hasn't
 * heard of yet — cash sales less their change, deposits, withdrawals. */
export function expectedCash(copy: TillCopy, userId: number, out: Action[]): string {
  let cash = num(copy.sessions.find((s) => s.userId === userId)?.expected ?? 0)
  for (const a of mine(out, userId)) {
    const b = a.body as { openingCash?: string; amount?: string; total?: string; payments?: Pay[] }
    if (a.kind === 'open') cash = num(b.openingCash)
    if (a.kind === 'close') cash = 0
    if (a.kind === 'deposit') cash += num(b.amount)
    if (a.kind === 'withdraw') cash -= num(b.amount)
    if (a.kind === 'sale') {
      const ps = b.payments ?? []
      const paid = ps.reduce((n, p) => n + num(p.amount), 0)
      const change = Math.max(0, paid - num(b.total))
      cash += ps.filter((p) => p.method === 'cash').reduce((n, p) => n + num(p.amount), 0) - change
    }
  }
  return dec(cash)
}

/** A hold that waits in the outbox; `ref` lets it be taken back before it goes. */
export type HeldRow = CopyHeld & { ref?: string }

/** This person's held sales: the copy's, less the ones resumed, plus holds not sent yet. */
export function heldList(copy: TillCopy, userId: number, out: Action[]): HeldRow[] {
  const gone = new Set(mine(out, userId).filter((a) => a.kind === 'unhold').map((a) => (a.body as { heldId: number }).heldId))
  const local = mine(out, userId).filter((a) => a.kind === 'hold').map((a): HeldRow => {
    const b = a.body as { label: string; saleType: string; customerId?: number; discount: string; cart: string; itemCount: number; total: string }
    return { id: 0, userId, label: b.label, saleType: b.saleType, customerId: b.customerId ?? 0, discount: b.discount, cart: b.cart, itemCount: b.itemCount, total: b.total, createdAt: a.at, ref: a.ref }
  })
  return [...copy.held.filter((h) => h.userId === userId && !gone.has(h.id)), ...local]
}
