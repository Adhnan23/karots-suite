import type { HeldSale } from '@/api/types.gen'
import type { CartLine, DiscountType, TillState } from './types'

type HeldBlob = { v: 2; lines: CartLine[]; billDiscountType: DiscountType; meta?: Record<string, unknown> }

/** What goes into held_sales.cart: an SPA-owned document (the server stores it opaquely). */
export const toHeldBlob = (s: TillState) =>
  JSON.stringify({ v: 2, lines: s.lines, billDiscountType: s.billDiscountType, meta: s.meta ?? {} } satisfies HeldBlob)

/** Re-key every line on the way back in (keys must stay unique in this cart). */
export function fromHeld(h: HeldSale, seq: number) {
  let parsed: unknown
  try {
    parsed = JSON.parse(h.cart)
  } catch {
    parsed = []
  }
  const blob = !Array.isArray(parsed) && parsed && typeof parsed === 'object' ? (parsed as Partial<HeldBlob>) : null
  const raw: CartLine[] = blob?.lines ?? []
  const lines = raw.map((l) => ({ ...l, key: ++seq }))
  return { lines, seq, billDiscountType: (blob?.billDiscountType === 'percent' ? 'percent' : 'fixed') as DiscountType, meta: blob?.meta ?? {} }
}
