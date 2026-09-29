import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { LotRow } from '@/api/types.gen'
import { INPUT } from '@/components/admin/fields'
import { PickerShell } from '@/components/admin/PickerShell'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { configQuery, errMsg } from '@/features/till/api'
import { money, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { lotsQuery } from './queries'

const DAY = 86_400_000
/** Days until a YYYY-MM-DD expiry (negative = expired). */
export const daysLeft = (ymd: string) => Math.ceil((new Date(`${ymd}T00:00:00`).getTime() - new Date().setHours(0, 0, 0, 0)) / DAY)

export function ExpiryTag({ ymd }: { ymd: string }) {
  if (!ymd) return null
  const d = daysLeft(ymd)
  return d < 0 ? <Tag tone="danger">expired</Tag> : d <= 30 ? <Tag tone="warn">{`${d}d left`}</Tag> : null
}

/** A product's live lots, oldest (next to sell) first, each with its own price
 * (karots-pos batch modal). Blank price = follows the product. */
export function LotsSheet({ productId, onClose, onAdjust }: { productId: number; onClose: () => void; onAdjust: () => void }) {
  const p = useQuery(lotsQuery(productId)).data
  const sym = useSuspenseQuery(configQuery).data.symbol
  const qc = useQueryClient()
  async function price(l: LotRow, v: string) {
    if (v === l.sellingPrice) return
    try {
      await api(`/admin/stock/batches/${l.id}/price`, { json: { sellingPrice: v } })
      toast.success(v ? `Lot priced at ${money(sym, v)}` : 'Lot follows the product price again')
      void qc.invalidateQueries({ queryKey: ['admin', 'lots', productId] })
      void qc.invalidateQueries({ queryKey: ['till'] })
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  return (
    <PickerShell fit open onOpenChange={(o) => !o && onClose()} title={p ? p.name : 'Lots'}>
      {p && (
        <>
          <p className="text-sm text-muted-foreground">
            <b className="text-foreground tabular-nums">{qtyText(p.stockQty)} {p.unitAbbr}</b> on hand in {p.lots.length} {p.lots.length === 1 ? 'lot' : 'lots'} · product price {money(sym, p.sellingPrice)}
          </p>
          <ul aria-label="Lots" className="-mx-1 flex min-h-0 flex-1 flex-col gap-1.5 overflow-auto px-1">
            {p.lots.map((l, i) => (
              <li key={l.id} className={cn('grid grid-cols-[minmax(0,1fr)_110px] items-center gap-2 rounded-[var(--radius-sm)] border border-border p-2.5', l.expiryDate && daysLeft(l.expiryDate) < 0 && 'border-danger')}>
                <span className="min-w-0">
                  <span className="flex flex-wrap items-center gap-1.5">
                    <b className="tabular-nums">{qtyText(l.qtyRemaining)} <small className="font-normal text-faint">of {qtyText(l.qtyReceived)}</small></b>
                    {i === 0 && <Tag tone="brand">next to sell</Tag>}
                    <ExpiryTag ymd={l.expiryDate} />
                  </span>
                  <small className="block text-faint">
                    {l.source.replace(/_/g, ' ')} · {new Date(l.createdAt).toLocaleDateString([], { day: 'numeric', month: 'short', year: '2-digit' })}
                    {l.expiryDate && ` · exp ${l.expiryDate}`} · cost {money(sym, l.costPrice)}
                  </small>
                </span>
                <input aria-label={`Price of lot ${i + 1}`} inputMode="decimal" defaultValue={l.sellingPrice} placeholder={money(sym, p.sellingPrice).replace(/^\S+\s/, '')}
                  onBlur={(e) => void price(l, e.target.value.trim())} onKeyDown={(e) => e.key === 'Enter' && e.currentTarget.blur()}
                  className={cn(INPUT, 'h-10 text-right tabular-nums')} />
              </li>
            ))}
            {p.lots.length === 0 && <li className="p-6 text-center text-sm text-faint">No live lots — nothing on hand.</li>}
          </ul>
          <p className="text-[12px] text-faint">A lot's own price shows at the till as a “which price?” choice. Leave it blank to follow the product.</p>
          <Button className="h-10 font-bold" onClick={onAdjust}>Adjust the count</Button>
        </>
      )}
    </PickerShell>
  )
}
