import type { TillProduct } from '@/api/types.gen'
import { num, qtyText, r2 } from '@/lib/money'
import type { CartLine, LinePack, SaleType } from './types'

/** A product's packs priced for this sale type (wholesale falls back to the selling price). */
export const linePacks = (p: TillProduct, saleType: SaleType): LinePack[] =>
  (p.packs ?? []).map((k) => ({
    id: k.id, name: k.name, size: num(k.size),
    price: saleType === 'wholesale' && num(k.wholesalePrice) > 0 ? num(k.wholesalePrice) : num(k.sellingPrice),
  }))

/** "1 Card", "2 Cards", "2 Boxes", "2 Sachets" — as Go sales.packPlural. */
export function packPlural(name: string, n: number) {
  if (n === 1) return name
  const low = name.toLowerCase()
  if (low.endsWith('s')) return name
  if (low.endsWith('x') || low.endsWith('ch') || low.endsWith('sh')) return `${name}es`
  return `${name}s`
}

/** Mirror of Go sales.PackGross: every full pack (largest first) at the pack price
 * when cheaper than its units, the rest at the unit price. label = the split as
 * handed over ("1 Card + 2"), '' when no pack fits. Both read packs.fixture.json. */
export function packGross(qty: number, unit: number, packs: LinePack[]) {
  let rest = qty
  let gross = 0
  const parts: string[] = []
  for (const k of [...packs].sort((a, b) => b.size - a.size)) {
    if (!(k.size > 0)) continue
    const n = Math.floor(rest / k.size + 1e-9)
    if (n <= 0) continue
    const asUnits = k.size * unit
    const price = k.price > 0 && k.price <= asUnits ? k.price : asUnits
    gross += n * price
    rest = Number((rest - n * k.size).toFixed(3))
    parts.push(`${n} ${packPlural(k.name, n)}`)
  }
  gross = r2(gross + rest * unit)
  if (parts.length && rest > 0) parts.push(qtyText(rest))
  return { gross, label: parts.join(' + ') }
}

/** True when the line prices by pack (a combo's lines keep their own deal). */
export const packed = (l: CartLine) => !!l.packs?.length && !l.isService && !l.group
