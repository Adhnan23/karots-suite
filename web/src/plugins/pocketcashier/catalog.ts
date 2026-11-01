import type { CopyProduct, TillCopy, TillProduct } from '@/api/types.gen'
import { num } from '@/lib/money'
import { slots } from '@/plugins/slots'

const LIMIT = 50

/** What the phone can ring up itself. A product a plugin's till menu claims (it
 * opens that plugin's panel on the desktop) and a service with no price wait for
 * plugin menus on the phone (release 2); sell them at the counter. */
export function sellable(p: TillProduct): boolean {
  if (p.isService && num(p.sellingPrice) <= 0) return false
  return !slots.tillMenu.some((m) => m.claims?.(p))
}

/** Name, local name or barcode start, any case — over the copy, so it works offline. */
export function search(copy: TillCopy | null, text: string, categoryId = 0): CopyProduct[] {
  const all = (copy?.products ?? []).filter((p) => (!categoryId || p.categoryId === categoryId) && sellable(p))
  const q = text.trim().toLocaleLowerCase()
  if (!q) return all.slice(0, LIMIT)
  const words = q.split(/\s+/)
  return all.filter((p) => {
    const hay = `${p.name} ${p.nameLocal}`.toLocaleLowerCase()
    return words.every((w) => hay.includes(w)) || p.barcode.startsWith(q) || p.barcodes.some((b) => b.startsWith(q))
  }).slice(0, LIMIT)
}

/** A scanned code: the product, and the pack when it was a pack's own code. */
export function byBarcode(copy: TillCopy | null, code: string): { product: CopyProduct; packId?: number } | null {
  const c = code.trim()
  if (!c) return null
  for (const p of copy?.products ?? []) {
    if (p.barcode === c || p.barcodes.includes(c)) return { product: p }
    const pack = p.packCodes.find((k) => k.barcode === c)
    if (pack) return { product: p, packId: pack.packId }
  }
  return null
}
