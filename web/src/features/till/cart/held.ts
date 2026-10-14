import type { HeldSale } from '@/api/types.gen'
import { num } from '@/lib/money'
import { MAX_STOCK } from './lines'
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
  const raw: CartLine[] = Array.isArray(parsed) ? parsed.map(legacyLine) : (blob?.lines ?? [])
  const lines = raw.map((l) => ({ ...l, key: ++seq }))
  return { lines, seq, billDiscountType: (blob?.billDiscountType === 'percent' ? 'percent' : 'fixed') as DiscountType, meta: blob?.meta ?? {} }
}

/** A karots-pos Alpine cart line (restored from a karots-pos backup). */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function legacyLine(x: any): CartLine {
  return {
    key: 0, productId: num(x.id), name: String(x.name ?? ''), unitPrice: num(x.unit_price), taxRate: num(x.tax_rate),
    qty: num(x.qty), stock: x.is_service ? MAX_STOCK : num(x.stock), isService: !!x.is_service, allowDecimal: !!x.allowDecimal,
    batchId: num(x.batch_id), batchRemaining: num(x.batch_remaining), batchLabel: String(x.batch_label ?? ''),
    allowOversell: !!x.allow_oversell, discount: String(x.discount ?? ''), discountType: x.discountType === 'percent' ? 'percent' : 'fixed',
    trackSerial: !!x.track_serial, warrantyMonths: num(x.warranty_months), serials: Array.isArray(x.serials) ? x.serials.map(String) : [],
    priceOverride: String(x.price_override ?? ''), description: String(x.description ?? ''),
    components: Array.isArray(x.components) ? x.components.map((c: { product_id: unknown; quantity: unknown }) => ({ productId: num(c.product_id), quantity: String(c.quantity) })) : [],
  }
}
