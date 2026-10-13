import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AddBatchRequest, AdjustRequest, ConsumeRequest, LotRow, MergeBatchesRequest, ProductLots } from '@/api/types.gen'
import { INPUT } from '@/components/admin/fields'
import { PickerShell } from '@/components/admin/PickerShell'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { configQuery, errMsg } from '@/features/till/api'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { AddBatchForm } from './AddBatchForm'
import { MergePrompt } from './MergeBatches'
import { lotsQuery } from './queries'

const DAY = 86_400_000
/** Days until a YYYY-MM-DD expiry (negative = expired). */
export const daysLeft = (ymd: string) => Math.ceil((new Date(`${ymd}T00:00:00`).getTime() - new Date().setHours(0, 0, 0, 0)) / DAY)

export function ExpiryTag({ ymd }: { ymd: string }) {
  if (!ymd) return null
  const d = daysLeft(ymd)
  return d < 0 ? <Tag tone="danger">expired</Tag> : d <= 30 ? <Tag tone="warn">{`${d}d left`}</Tag> : null
}

type Edit = { id: number; mode: 'count' | 'remove' } | null

/** "Batches of …": a product's live lots, oldest (next to sell) first. Each has
 * its own Batch price (blank = follows the product), a Set count (tops the lot
 * up or takes the shortfall off it) and Remove (not there, or written off).
 * + Add batch types one in; Merge batches folds ticked ones into the oldest. */
export function LotsSheet({ productId, onClose }: { productId: number; onClose: () => void }) {
  const p = useQuery(lotsQuery(productId)).data
  const sym = useSuspenseQuery(configQuery).data.symbol
  const qc = useQueryClient()
  const [edit, setEdit] = useState<Edit>(null)
  const [count, setCount] = useState('')
  const [busy, setBusy] = useState(false)
  const [tool, setTool] = useState<'add' | 'merge' | null>(null)
  const [ticked, setTicked] = useState<number[]>([])
  const refresh = () => {
    void qc.invalidateQueries({ queryKey: ['admin'] })
    void qc.invalidateQueries({ queryKey: ['till'] })
  }
  async function price(l: LotRow, v: string) {
    if (v === l.sellingPrice) return
    try {
      await api(`/admin/stock/batches/${l.id}/price`, { json: { sellingPrice: v } })
      toast.success(v ? `Batch priced at ${money(sym, v)}` : 'Batch follows the product price again')
      refresh()
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  async function run(msg: string, fn: () => Promise<unknown>) {
    setBusy(true)
    try {
      await fn()
      toast.success(msg)
      setEdit(null)
      refresh()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  /** Add / merge answer the product's batches as they now stand. */
  const write = (msg: string, path: string, body: AddBatchRequest | MergeBatchesRequest) => run(msg, async () => {
    qc.setQueryData(lotsQuery(productId).queryKey, await api<ProductLots>(`/admin/stock/lots/${productId}/${path}`, { json: body }))
    setTool(null)
    setTicked([])
  })
  const tick = (id: number) => setTicked((t) => (t.includes(id) ? t.filter((x) => x !== id) : [...t, id]))
  /** The product total that leaves this lot at n (the adjust API takes the product's count). */
  const totalWith = (l: LotRow, n: number) => String(+(num(p!.stockQty) - num(l.qtyRemaining) + n).toFixed(3))
  const setLotCount = (l: LotRow) => {
    const body: AdjustRequest = { productId, newQuantity: totalWith(l, num(count)), note: 'Batch count', batchId: l.id }
    return run(`Batch set to ${qtyText(count)} ${p!.unitAbbr}`, () => api('/admin/stock/adjust', { json: body }))
  }
  const removeLot = (l: LotRow, writeOff: boolean) => {
    const done = `${qtyText(l.qtyRemaining)} ${p!.unitAbbr} ${writeOff ? 'written off' : 'taken off the count'}`
    if (writeOff) {
      const body: ConsumeRequest = { productId, quantity: l.qtyRemaining, reason: 'damage', note: 'Batch removed', batchId: l.id }
      return run(done, () => api('/admin/stock/damage', { json: body }))
    }
    const body: AdjustRequest = { productId, newQuantity: totalWith(l, 0), note: 'Batch not there', batchId: l.id }
    return run(done, () => api('/admin/stock/adjust', { json: body }))
  }
  const open = (id: number, mode: 'count' | 'remove', l: LotRow) => {
    setCount(mode === 'count' ? qtyText(l.qtyRemaining) : '')
    setEdit(edit?.id === id && edit.mode === mode ? null : { id, mode })
  }
  return (
    <PickerShell fit open onOpenChange={(o) => !o && onClose()} title={p ? `Batches of ${p.name}` : 'Batches'}>
      {p && (
        <>
          <p className="text-sm text-muted-foreground">
            <b className="text-foreground tabular-nums">{qtyText(p.stockQty)} {p.unitAbbr}</b> on hand in {p.lots.length} {p.lots.length === 1 ? 'batch' : 'batches'} · product price {money(sym, p.sellingPrice)}
          </p>
          <ul aria-label="Batches" className="-mx-1 flex min-h-0 flex-1 flex-col gap-1.5 overflow-auto px-1">
            {tool === 'add' && (
              <li><AddBatchForm cost="product's" price={money(sym, p.sellingPrice).replace(/^\S+\s/, '')} busy={busy}
                onCancel={() => setTool(null)} onSave={(b) => void write(`${qtyText(b.qty)} ${p.unitAbbr} added as a new batch`, 'add', b)} /></li>
            )}
            {p.lots.map((l, i) => (
              <li key={l.id} aria-label={`Batch ${i + 1}`} className={cn('flex flex-col gap-2 rounded-[var(--radius-sm)] border border-border p-2.5', l.expiryDate && daysLeft(l.expiryDate) < 0 && 'border-danger', ticked.includes(l.id) && 'border-brand')}>
                <div className="grid grid-cols-[minmax(0,1fr)_110px] items-end gap-2">
                  <span className="flex min-w-0 items-start gap-2">
                    {tool === 'merge' && (
                      <input type="checkbox" aria-label={`Merge batch ${i + 1}`} checked={ticked.includes(l.id)} onChange={() => tick(l.id)} className="mt-1 size-4 shrink-0 accent-[var(--brand)]" />
                    )}
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
                  </span>
                  <label className="flex flex-col gap-0.5 text-[11px] font-semibold text-muted-foreground">
                    Batch price
                    <input aria-label={`Batch price ${i + 1}`} inputMode="decimal" defaultValue={l.sellingPrice} placeholder={money(sym, p.sellingPrice).replace(/^\S+\s/, '')}
                      onBlur={(e) => void price(l, e.target.value.trim())} onKeyDown={(e) => e.key === 'Enter' && e.currentTarget.blur()}
                      className={cn(INPUT, 'h-10 text-right text-sm font-normal text-foreground tabular-nums')} />
                  </label>
                </div>
                {tool !== 'merge' && <div className="flex gap-1.5">
                  <Button variant="outline" size="sm" aria-pressed={edit?.id === l.id && edit.mode === 'count'} onClick={() => open(l.id, 'count', l)}>Set count</Button>
                  <Button variant="ghost" size="sm" className="text-danger" aria-pressed={edit?.id === l.id && edit.mode === 'remove'} onClick={() => open(l.id, 'remove', l)}>Remove batch</Button>
                </div>}
                {edit?.id === l.id && edit.mode === 'count' && (
                  <form className="flex items-center gap-2 rounded-[var(--radius-sm)] bg-muted p-2" onSubmit={(e) => { e.preventDefault(); void setLotCount(l) }}>
                    <span className="flex-1 text-sm">Actually in this batch</span>
                    <input autoFocus aria-label="Batch count" inputMode="decimal" value={count} onChange={(e) => setCount(e.target.value.replace(/[^\d.]/g, ''))}
                      className={cn(INPUT, 'h-10 w-24 text-right font-bold tabular-nums')} />
                    <Button type="submit" size="sm" disabled={busy || count === '' || num(count) === num(l.qtyRemaining)}>Save</Button>
                  </form>
                )}
                {edit?.id === l.id && edit.mode === 'remove' && (
                  <div className="flex flex-col gap-1.5 rounded-[var(--radius-sm)] bg-muted p-2 text-sm">
                    <span>Take all {qtyText(l.qtyRemaining)} {p.unitAbbr} of this batch off — why?</span>
                    <div className="flex flex-wrap gap-1.5">
                      <Button size="sm" variant="outline" disabled={busy} onClick={() => void removeLot(l, false)}>Not there (count correction)</Button>
                      <Button size="sm" variant="destructive" disabled={busy} onClick={() => void removeLot(l, true)}>Write off (damaged / expired)</Button>
                    </div>
                  </div>
                )}
              </li>
            ))}
            {p.lots.length === 0 && tool !== 'add' && <li className="p-6 text-center text-sm text-faint">No live batches — nothing on hand.</li>}
          </ul>
          {tool === 'merge' && ticked.length >= 2 && (
            <MergePrompt key={ticked.join()} lots={p.lots.filter((l) => ticked.includes(l.id))} unit={p.unitAbbr} sym={sym} productPrice={p.sellingPrice} busy={busy}
              onCancel={() => { setTool(null); setTicked([]) }} onMerge={(r) => void write(`${r.batchIds.length} batches merged`, 'merge', r)} />
          )}
          {tool === 'merge' && ticked.length < 2 && <p className="text-sm text-muted-foreground">Tick the batches to merge — they join the oldest one.</p>}
          {tool === null && (
            <div className="flex flex-wrap gap-1.5">
              <Button variant="outline" size="sm" onClick={() => { setEdit(null); setTool('add') }}>+ Add batch</Button>
              {p.lots.length >= 2 && <Button variant="outline" size="sm" onClick={() => { setEdit(null); setTool('merge') }}>Merge batches</Button>}
            </div>
          )}
          {tool === 'merge' && ticked.length < 2 && <Button variant="ghost" size="sm" className="self-start" onClick={() => setTool(null)}>Cancel merge</Button>}
          <p className="text-[12px] text-faint">A batch's own price shows at the till as a “which price?” choice. Leave it blank to follow the product.</p>
        </>
      )}
    </PickerShell>
  )
}
