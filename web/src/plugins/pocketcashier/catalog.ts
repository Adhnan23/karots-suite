import type { CopyProduct, TillCopy } from '@/api/types.gen'

const LIMIT = 50

/** Name, local name or barcode start, any case — over the copy, so it works offline. */
export function search(copy: TillCopy | null, text: string, categoryId = 0): CopyProduct[] {
  const all = (copy?.products ?? []).filter((p) => !categoryId || p.categoryId === categoryId)
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
