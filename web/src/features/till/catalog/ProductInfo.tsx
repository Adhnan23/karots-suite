import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { ProductDetail, TillProduct } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { money, qtyText } from '@/lib/money'
import { configQuery } from '../api'
import { useTillUi } from '../TillUi'
import { useAddProduct } from './useAddProduct'

/** ⓘ popup: what the card knows at once, then /detail (cost only for allowed users). */
export function ProductInfo({ product }: { product: TillProduct }) {
  const cfg = useSuspenseQuery(configQuery).data
  const ui = useTillUi()
  const add = useAddProduct()
  const d = useQuery({ queryKey: ['till', 'detail', product.id], queryFn: () => api<ProductDetail>(`/till/products/${product.id}/detail`) }).data
  const rows: [string, string][] = [
    ['Price', money(cfg.symbol, d?.sellingPrice ?? product.sellingPrice)],
    ['In stock', product.isService ? 'service' : `${qtyText(d?.stockQty ?? product.stockQty)} ${product.unitAbbr}`],
  ]
  if (d?.category) rows.push(['Category', d.category])
  if (d?.barcode) rows.push(['Barcode', d.barcode])
  if (d?.warranty) rows.push(['Warranty', d.warranty])
  if (d?.showCost) {
    rows.push(['Cost', money(cfg.symbol, d.costPrice)])
    if (d.marginPct) rows.push(['Margin', `${d.marginPct}%`])
  }
  for (const r of d?.rows ?? []) rows.push([r.label, r.value])
  return (
    <Dialog open onOpenChange={(o) => !o && ui.close()}>
      <DialogContent className="sm:max-w-[420px]">
        <DialogHeader>
          <DialogTitle>{product.name}</DialogTitle>
          <DialogDescription>{d ? 'Product details' : 'Loading details…'}</DialogDescription>
        </DialogHeader>
        <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-1.5 text-[14px]">
          {rows.map(([k, v]) => (
            <div key={k} className="contents"><dt className="text-muted-foreground">{k}</dt><dd className="text-right font-semibold tabular-nums">{v}</dd></div>
          ))}
        </dl>
        <DialogFooter>
          <Button variant="ghost" onClick={ui.close}>Close</Button>
          <Button autoFocus onClick={() => { ui.close(); add(product) }}>Add to cart</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
