import { dec, num } from '@/lib/money'
import type { Action } from '../../store/outbox'
import type { OfflineRoute, RouteCtx } from '../fetch'

type Float = { id: number; balance: string }
type Part = { floats: Float[] }
type SaleItem = { priceOverride?: string; quantity?: string; plugin?: { recharge?: { kind?: string; device?: number } } }

/** A float's change from sales waiting on the phone: money sent out lowers it; a
 * customer's e-money coming in raises it. A package's float cost is only on the PC,
 * so its price stands in until the copy refreshes. */
function floatMoves(out: Action[]): Map<number, number> {
  const moves = new Map<number, number>()
  for (const a of out) {
    if (a.kind !== 'sale') continue
    for (const it of (a.body as { items?: SaleItem[] }).items ?? []) {
      const tag = it.plugin?.recharge
      if (!tag?.device || tag.kind === 'charge') continue
      const amount = num(it.priceOverride) * (num(it.quantity) || 1)
      moves.set(tag.device, (moves.get(tag.device) ?? 0) + (tag.kind === 'cashout' ? amount : -amount))
    }
  }
  return moves
}

function till(c: RouteCtx) {
  const part = c.parts.recharge as Part | undefined
  if (!part) return null
  const moves = floatMoves(c.out)
  return { ...part, floats: part.floats.map((f) => (moves.has(f.id) ? { ...f, balance: dec(num(f.balance) + moves.get(f.id)!) } : f)) }
}

type PackageIn = { carrierId?: number; name?: string; price?: string }
type CountsIn = { counts?: { deviceId: number; counted: string }[] }

export const rechargeRoutes: OfflineRoute[] = [
  { method: 'GET', path: '/x/recharge/till', answer: till },
  // Sale lines aren't in the part: a return offline has no float field, and the PC's default applies.
  { method: 'GET', path: '/x/recharge/sale-lines', answer: () => [] },
  { method: 'POST', path: '/x/recharge/packages/quick', queue: true,
    label: (c) => `New package: ${(c.body as PackageIn).name}`,
    answer: (c) => {
      const b = c.body as PackageIn
      return { package: { id: `temp:${c.ref}`, carrierId: b.carrierId, name: b.name, price: b.price, floatCost: '0', isActive: true, sort: 0, addedBy: '' }, created: true }
    } },
  { method: 'POST', path: '/x/recharge/counts/drawer', queue: true,
    label: () => 'Float counts at closing',
    answer: (c) => {
      const part = till(c)
      return ((c.body as CountsIn).counts ?? []).map((x) => ({ deviceId: x.deviceId, expected: part?.floats.find((f) => f.id === x.deviceId)?.balance ?? '0.00', counted: x.counted }))
    } },
]
