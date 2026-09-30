import type { ReorderLine } from '@/api/types.gen'

/** One ticked item waiting to be ordered. */
export type CartItem = ReorderLine & { name: string; unitAbbr: string }
export type Cart = Record<number, CartItem>

const KEY = 'ks-reorder-cart'

/** The reorder cart outlives filter changes and reloads (karots-pos
 * reorder_cart_v1): a per-browser convenience, so storage failures are ignored. */
export function loadCart(): Cart {
  try {
    return JSON.parse(localStorage.getItem(KEY) ?? '{}') ?? {}
  } catch {
    return {}
  }
}
export function saveCart(c: Cart) {
  try {
    localStorage.setItem(KEY, JSON.stringify(c))
  } catch {
    /* private window / blocked storage: the cart just won't survive a reload */
  }
}

/** How many orders the cart becomes (one per supplier) and what's still missing one. */
export function cartSummary(c: Cart) {
  const items = Object.values(c)
  return { items: items.length, suppliers: new Set(items.filter((i) => i.supplierId).map((i) => i.supplierId)).size, noSupplier: items.filter((i) => !i.supplierId).length }
}
