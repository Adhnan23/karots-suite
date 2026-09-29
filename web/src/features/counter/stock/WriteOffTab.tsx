import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { Lot, TillProduct, WriteOffRequest } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, errMsg } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { ProductFind } from '../ProductFind'
import { CounterPage } from '../ui'

const REASONS = [
  { v: 'damage', label: 'Damaged / expired', sub: 'broken, spoilt, past date' },
  { v: 'own_use', label: 'Shop use', sub: 'used in the shop' },
  { v: 'staff', label: 'Staff', sub: 'given to staff' },
] as const
const DONE: Record<string, string> = { damage: 'Written off', own_use: 'Recorded as shop use', staff: 'Recorded as staff welfare' }
const LABEL = 'flex flex-col gap-1.5 text-[13px] font-semibold text-muted-foreground'

/** Take stock out for anything that isn't a sale. The lot matters: it sets the cost
 * booked and what the till keeps offering. */
export function WriteOffTab() {
  const cfg = useSuspenseQuery(configQuery).data
  const [p, setP] = useState<TillProduct | null>(null)
  return (
    <CounterPage aside={p ? <WriteOffForm key={p.id} p={p} sym={cfg.symbol} onDone={() => setP(null)} /> : <Empty />} flush>
      <ProductFind title="Write-off" sym={cfg.symbol} picked={p?.id ?? null} onPick={setP} filter={(x) => !x.isService} />
    </CounterPage>
  )
}

function Empty() {
  return <p className="m-auto max-w-[260px] p-8 text-center text-faint">Pick a product on the left to take it out of stock.</p>
}

function WriteOffForm({ p, sym, onDone }: { p: TillProduct; sym: string; onDone: () => void }) {
  const qc = useQueryClient()
  const lots = useQuery({ queryKey: ['till', 'lots', p.id], queryFn: () => api<Lot[]>(`/till/products/${p.id}/lots`) }).data ?? []
  const [lot, setLot] = useState(0) // 0 = oldest first (FEFO)
  const [reason, setReason] = useState<string>('damage')
  const [qty, setQty] = useState('1')
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const max = lot ? num(lots.find((l) => l.batchId === lot)?.qtyRemaining) : num(p.stockQty)

  async function submit(e: { preventDefault(): void }) {
    e.preventDefault()
    if (!(num(qty) > 0)) return void toast.error('Enter how many')
    setBusy(true)
    try {
      const body: WriteOffRequest = { productId: p.id, batchId: lot, quantity: qty, note, reason }
      await api('/till/write-off', { json: body })
      toast.success(`${DONE[reason]} — ${qtyText(qty)} ${p.unitAbbr} ${p.name}`)
      void qc.invalidateQueries({ queryKey: ['till'] })
      onDone()
    } catch (err) {
      toast.error(errMsg(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={submit} autoComplete="off" className="flex flex-col gap-5 overflow-auto p-5">
      <div>
        <h2 className="font-display text-xl font-bold">{p.name}</h2>
        <p className="text-[13px] text-muted-foreground tabular-nums">{qtyText(p.stockQty)} {p.unitAbbr} on hand · {money(sym, p.sellingPrice)}</p>
      </div>
      <div role="radiogroup" aria-label="Reason" className="grid grid-cols-3 gap-1.5">
        {REASONS.map((r) => (
          <button key={r.v} type="button" role="radio" aria-checked={reason === r.v} onClick={() => setReason(r.v)}
            className="flex flex-col items-start rounded-[var(--radius-sm)] border border-border bg-muted px-3 py-2 text-left aria-checked:border-brand aria-checked:bg-brand aria-checked:text-brand-foreground">
            <b className="text-[13px]">{r.label}</b>
            <small className="text-[11.5px] opacity-75">{r.sub}</small>
          </button>
        ))}
      </div>
      {lots.length > 1 && (
        <div className={LABEL}>
          Which lot?
          <div role="radiogroup" aria-label="Lot" className="flex flex-col gap-1">
            {[{ batchId: 0, label: 'Oldest first', sub: 'let the system pick' },
              ...lots.map((l) => ({ batchId: l.batchId, label: `${l.batchNo || `Lot ${l.batchId}`} · ${qtyText(l.qtyRemaining)} left`,
                sub: [l.expiryDate && `exp ${l.expiryDate}`, `${money(sym, l.price)}`].filter(Boolean).join(' · ') }))].map((o) => (
              <button key={o.batchId} type="button" role="radio" aria-checked={lot === o.batchId} onClick={() => setLot(o.batchId)}
                className="flex items-center justify-between rounded-[var(--radius-sm)] border border-border px-3 py-2 text-left text-[13px] text-foreground aria-checked:border-brand aria-checked:bg-selected aria-checked:text-selected-foreground">
                <b>{o.label}</b><small className="opacity-75 tabular-nums">{o.sub}</small>
              </button>
            ))}
          </div>
        </div>
      )}
      <label className={LABEL}>
        How many? <small className="font-normal">up to {qtyText(max)} {p.unitAbbr}</small>
        <input aria-label="Quantity" inputMode="decimal" value={qty} onChange={(e) => setQty(e.target.value.replace(/[^\d.]/g, ''))}
          className={cn(FIELD, 'h-12 font-display text-xl font-bold tabular-nums')} />
      </label>
      <label className={LABEL}>
        Note
        <input aria-label="Note" placeholder="What happened (optional)" value={note} onChange={(e) => setNote(e.target.value)} className={FIELD} />
      </label>
      <Button type="submit" size="lg" disabled={busy} className="h-12 text-[15px]">
        Take {qtyText(num(qty) || 0)} {p.unitAbbr} out of stock
      </Button>
    </form>
  )
}
