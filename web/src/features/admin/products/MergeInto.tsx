import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { ArrowRight, Merge, TriangleAlert } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { MergePreview, MergeRequest } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import { configQuery, errMsg } from '@/features/till/api'
import { money, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { ProductPicker } from './ProductPicker'

const plural = (n: number, one: string, many = `${one}s`) => `${n} ${n === 1 ? one : many}`

/** "Merge into…": pick the product to keep, then a confirm that shows what moves (barcodes,
 * stock, sales) or why it can't. Not undoable — the confirm says so. */
export function MergeInto({ from, trigger, onDone }: {
  from: { id: number; name: string }
  trigger: (open: () => void) => ReactNode
  onDone: () => void
}) {
  const [into, setInto] = useState<{ id: number; name: string } | null>(null)
  return (
    <>
      <ProductPicker title={`Merge ${from.name} into…`} exclude={new Set([from.id])} onPick={(p) => setInto(p)} trigger={trigger} />
      {into && <MergeConfirm from={from} into={into} onClose={() => setInto(null)} onDone={onDone} />}
    </>
  )
}

/** The merge confirm on its own — for a caller that already knows both products. */
export function MergeConfirm({ from, into, onClose, onDone }: { from: { id: number; name: string }; into: { id: number; name: string }; onClose: () => void; onDone: () => void }) {
  const sym = useSuspenseQuery(configQuery).data.symbol
  const qc = useQueryClient()
  const path = `/admin/products/${from.id}/merge/${into.id}`
  const pv = useQuery({ queryKey: ['admin', 'merge', from.id, into.id], queryFn: () => api<MergePreview>(path), gcTime: 0 }).data
  const [lotPrice, setLotPrice] = useState<MergeRequest['lotPrice']>('kept')
  const [busy, setBusy] = useState(false)
  async function merge() {
    setBusy(true)
    try {
      await api(path, { json: { lotPrice } satisfies MergeRequest })
      toast.success(`${from.name} merged into ${into.name}`)
      void qc.invalidateQueries({ queryKey: ['admin'] })
      void qc.invalidateQueries({ queryKey: ['till'] })
      onClose()
      onDone()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  const facts = pv && [
    `${plural(pv.barcodes, 'barcode')} — scanning ${pv.barcodes === 1 ? 'it' : 'them'} finds ${pv.intoName}`,
    `${qtyText(pv.stock)} in stock${pv.lots > 1 ? ` (${pv.lots} lots)` : ''}, added to ${pv.intoName}'s`,
    pv.sales + pv.purchases > 0 && `${plural(pv.sales, 'sale')} and ${plural(pv.purchases, 'purchase')} — reports show them under ${pv.intoName}`,
  ].filter((f): f is string => !!f)
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="gap-3 p-5 sm:max-w-md">
        <DialogTitle className="font-display text-lg font-bold">Merge into {into.name}?</DialogTitle>
        <DialogDescription asChild>
          <div className="flex flex-col gap-3 text-[14px] text-foreground">
            <p className="flex flex-wrap items-center gap-x-2 gap-y-1 rounded-[var(--radius-sm)] bg-muted px-3 py-2 font-semibold">
              <span className="text-muted-foreground line-through decoration-muted-foreground/60">{from.name}</span>
              <ArrowRight size={15} className="text-faint" />
              <span>{into.name}</span>
            </p>
            {!pv ? <p className="text-muted-foreground">Counting what moves…</p> : pv.refusal ? (
              <p role="alert" className="flex gap-2 rounded-[var(--radius-sm)] bg-[color-mix(in_srgb,var(--warning)_14%,transparent)] px-3 py-2 text-[13px]">
                <TriangleAlert size={16} className="mt-0.5 flex-none text-warning" /> {pv.refusal}
              </p>
            ) : (
              <>
                <ul className="flex flex-col gap-1.5 text-[13px]">
                  {facts?.map((f) => <li key={f} className="flex gap-2"><span aria-hidden className="mt-[7px] h-1.5 w-1.5 flex-none rounded-full bg-brand" />{f}</li>)}
                </ul>
                {pv.priceClash && (
                  <fieldset className="flex flex-col gap-1.5">
                    <legend className="mb-1 text-[13px] font-semibold text-muted-foreground">Its stock sells at {money(sym, pv.fromPrice)}; {pv.intoName} at {money(sym, pv.intoPrice)}</legend>
                    {([['kept', `Sell all at ${money(sym, pv.intoPrice)}`], ['keep', `Keep ${money(sym, pv.fromPrice)} on those ${qtyText(pv.stock)} (the till asks which price)`]] as const).map(([v, label]) => (
                      <label key={v} className={cn('flex cursor-pointer items-center gap-2 rounded-[var(--radius-sm)] border px-3 py-2 text-[13px]', lotPrice === v ? 'border-selected bg-selected/10 font-semibold' : 'border-border')}>
                        <input type="radio" name="lot-price" checked={lotPrice === v} onChange={() => setLotPrice(v)} className="accent-[var(--selected)]" /> {label}
                      </label>
                    ))}
                  </fieldset>
                )}
                <p className="text-[13px] text-muted-foreground">{from.name} leaves the catalog. This can’t be undone.</p>
              </>
            )}
          </div>
        </DialogDescription>
        <div className="mt-1 flex justify-end gap-2">
          <Button type="button" variant="ghost" className="h-10 px-4" onClick={onClose}>{pv?.refusal ? 'Close' : 'Keep both'}</Button>
          {pv && !pv.refusal && (
            <Button type="button" disabled={busy} className="h-10 gap-1.5 bg-danger px-4 font-bold text-white hover:bg-danger/90" onClick={() => void merge()}>
              <Merge size={15} /> Merge
            </Button>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
