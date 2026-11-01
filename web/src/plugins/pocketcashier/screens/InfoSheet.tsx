import { useEffect, useState } from 'react'
import { api } from '@/api/client'
import type { CopyProduct, ProductDetail, TillCopy } from '@/api/types.gen'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { linePacks, packGross } from '@/features/till/cart/packs'
import { money, num, qtyText } from '@/lib/money'

/** ⓘ on the phone: what the copy's card knows, then the PC's /detail while live
 * (the server leaves cost out for people not allowed to see it). */
export function InfoSheet({ product: p, copy, live, onAdd, onClose }: {
  product: CopyProduct
  copy: TillCopy
  live: boolean
  onAdd: () => void
  onClose: () => void
}) {
  const sym = copy.symbol
  const [d, setD] = useState<ProductDetail | null>(null)
  useEffect(() => {
    if (!live) return
    let on = true
    api<ProductDetail>(`/till/products/${p.id}/detail`).then((r) => on && setD(r), () => {})
    return () => { on = false }
  }, [live, p.id])
  const stock = d?.stockQty ?? p.stockQty
  const rows: [string, string][] = [
    ['Price', money(sym, d?.sellingPrice ?? p.sellingPrice)],
    ['In stock', p.isService ? 'service' : `${qtyText(stock)} ${p.unitAbbr}${live ? '' : ' (copy)'}`],
  ]
  if (num(p.wholesalePrice) > 0) rows.push(['Wholesale', money(sym, p.wholesalePrice)])
  for (const k of p.packs) {
    rows.push([`${k.name} ×${qtyText(k.size)}`, num(k.sellingPrice) > 0 ? money(sym, k.sellingPrice) : `${money(sym, num(k.size) * num(p.sellingPrice))} (follows the unit price)`])
  }
  if (p.packs.length && !p.isService) rows[1][1] += ` = ${packGross(num(stock), 0, linePacks(p, 'retail')).label || 'less than a pack'}`
  if (d?.category) rows.push(['Category', d.category])
  if (d?.barcode || p.barcode) rows.push(['Barcode', d?.barcode || p.barcode])
  if (d?.warranty) rows.push(['Warranty', d.warranty])
  else if (p.warrantyMonths > 0) rows.push(['Warranty', `${p.warrantyMonths} months`])
  if (d?.showCost) {
    rows.push(['Cost', money(sym, d.costPrice)])
    if (d.marginPct) rows.push(['Margin', `${d.marginPct}%`])
  }
  for (const r of d?.rows ?? []) rows.push([r.label, r.value])
  if (p.badges.length) rows.push(['Notes', p.badges.join(' · ')])

  return (
    <Sheet open onOpenChange={(o) => !o && onClose()}>
      <SheetContent side="bottom" className="max-h-[85dvh] gap-3 overflow-y-auto p-4">
        <SheetHeader className="p-0">
          <SheetTitle>{p.name}</SheetTitle>
          <SheetDescription>{live ? (d ? 'Product details' : 'Loading details…') : 'From the copy on this phone'}</SheetDescription>
        </SheetHeader>
        <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-1.5 text-[14px]">
          {rows.map(([k, v]) => (
            <div key={k} className="contents"><dt className="text-muted-foreground">{k}</dt><dd className="text-right font-semibold tabular-nums">{v}</dd></div>
          ))}
        </dl>
        <button type="button" onClick={() => { onClose(); onAdd() }} className="rounded-[var(--radius)] bg-brand p-3.5 text-base font-extrabold text-brand-foreground">Add to cart</button>
      </SheetContent>
    </Sheet>
  )
}
