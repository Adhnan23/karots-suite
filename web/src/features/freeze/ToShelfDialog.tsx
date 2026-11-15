import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { conversionsQuery } from '@/features/counter/queries'
import { errMsg } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { refreshStock, tillProductQuery } from './api'

/** "Moved to the shelf": a frozen product's box opened onto the front shelf. It
 * follows how the product is set up: the box side of a conversion runs it (the
 * singles get them only if they aren't frozen); otherwise a quantity, or a pack.
 * Never an adjustment, never a loss. */
export function ToShelfDialog({ id, onClose }: { id: number; onClose: () => void }) {
  const qc = useQueryClient()
  const p = useQuery(tillProductQuery(id)).data
  const conv = (useQuery(conversionsQuery).data ?? []).find((r) => r.fromId === id)
  const [qty, setQty] = useState('1')
  const [pack, setPack] = useState(0) // 0 = single units
  const [busy, setBusy] = useState(false)
  if (!p) return null
  const size = num(p.packs.find((k) => k.id === pack)?.size) || 1
  const units = num(qty) * size
  async function save(e: { preventDefault(): void }) {
    e.preventDefault()
    if (!(units > 0)) return
    setBusy(true)
    try {
      if (conv) await api(`/till/conversions/${conv.id}/run`, { json: { quantity: qty } })
      else await api(`/till/stock/${id}/to-shelf`, { json: { quantity: String(units) } })
      toast.success(conv ? `Opened ${qtyText(qty)} ${p!.name} → ${conv.toName}` : `${qtyText(units)} ${p!.unitAbbr} ${p!.name} moved to the shelf`)
      refreshStock(qc)
      onClose()
    } catch (err) {
      toast.error(errMsg(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-[420px]">
        <form onSubmit={save} autoComplete="off" className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>Moved to the shelf</DialogTitle>
            <DialogDescription>
              {conv
                ? `Opens ${p.name} into ${conv.toName} (1 → ${qtyText(conv.ratio)}). ${p.name} comes out of the counted stock.`
                : `${p.name} comes out of the counted stock (${qtyText(p.stockQty)} ${p.unitAbbr}) and goes on the shelf, which isn't counted while frozen.`}
            </DialogDescription>
          </DialogHeader>
          {!conv && p.packs.length > 0 && (
            <div role="radiogroup" aria-label="How" className="flex flex-wrap gap-1.5">
              {[{ id: 0, name: p.unitAbbr, size: '1' }, ...p.packs].map((k) => (
                <button key={k.id} type="button" role="radio" aria-checked={pack === k.id} onClick={() => setPack(k.id)}
                  className="rounded-pill border border-border px-3 py-1.5 text-[13px] font-semibold aria-checked:border-selected aria-checked:bg-selected aria-checked:text-selected-foreground">
                  {k.id ? `${k.name} ×${qtyText(k.size)}` : `Single ${k.name}`}
                </button>
              ))}
            </div>
          )}
          <label className="flex flex-col gap-1.5 text-[13px] font-semibold text-muted-foreground">
            How many {conv ? p.unitAbbr : pack ? 'packs' : p.unitAbbr}?
            {size > 1 && <small className="font-normal">= {qtyText(units)} {p.unitAbbr}</small>}
            <input autoFocus aria-label="How many" inputMode="decimal" value={qty} onChange={(e) => setQty(e.target.value.replace(/[^\d.]/g, ''))}
              className={cn(FIELD, 'h-12 font-display text-xl font-bold tabular-nums')} />
          </label>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={busy || !(units > 0)}>{conv ? 'Open it' : 'Move to the shelf'}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
