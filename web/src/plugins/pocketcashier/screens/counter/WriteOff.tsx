import { Search } from 'lucide-react'
import { useState } from 'react'
import type { CopyProduct } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { FIELD } from '@/features/till/ui'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { CARD } from '../../bits'
import { search } from '../../catalog'
import { sendCall } from '../../sync'
import { CounterShell, type CounterProps } from './CounterShell'

const REASONS = [
  { v: 'damage', label: 'Damaged / expired' },
  { v: 'own_use', label: 'Shop use' },
  { v: 'staff', label: 'Staff' },
] as const
const DONE: Record<string, string> = { damage: 'Written off', own_use: 'Recorded as shop use', staff: 'Recorded as staff use' }

/** Take stock out for anything that isn't a sale, on or off the PC. */
export function WriteOff(p: CounterProps) {
  const [q, setQ] = useState('')
  const [picked, setPicked] = useState<CopyProduct | null>(null)
  const [notice, setNotice] = useState('')
  if (picked) return <Form key={picked.id} {...p} product={picked} onDone={(n) => { setNotice(n); setPicked(null); setQ(''); p.onSent() }} onBack={() => setPicked(null)} />
  const rows = q.trim() ? search(p.copy, q).filter((x) => !x.isService).slice(0, 30) : []
  return (
    <CounterShell title="Write-off" onBack={p.onBack}>
      {notice && <p role="status" className="px-1 text-sm font-semibold text-success">{notice}</p>}
      <div className="relative">
        <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
        <input aria-label="Find a product" placeholder="Name or barcode" autoComplete="off" value={q} onChange={(e) => setQ(e.target.value)} className={cn(FIELD, 'h-12 w-full pl-9')} />
      </div>
      {rows.map((x) => (
        <button key={x.id} type="button" onClick={() => { setNotice(''); setPicked(x) }} className={cn(CARD, 'flex min-h-14 items-center justify-between gap-3 px-4 py-2.5 text-left')}>
          <b className="min-w-0 truncate text-[15px]">{x.name}</b>
          <span className="shrink-0 text-[13px] font-semibold tabular-nums">{qtyText(x.stockQty)} {x.unitAbbr}</span>
        </button>
      ))}
    </CounterShell>
  )
}

function Form({ who, copy, product: x, onDone, onBack }: CounterProps & { product: CopyProduct; onDone: (n: string) => void }) {
  const [reason, setReason] = useState<string>('damage')
  const [lot, setLot] = useState(0) // 0 = oldest first
  const [qty, setQty] = useState('1')
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const lots = copy.priceOptions?.[String(x.id)] ?? []
  const shown = lot ? num(lots.find((l) => l.batchId === lot)?.qtyRemaining) : num(x.stockQty)
  const over = num(qty) > shown

  async function submit(e: { preventDefault(): void }) {
    e.preventDefault()
    setBusy(true)
    setError('')
    const what = `${qtyText(qty)} ${x.unitAbbr} ${x.name}`
    const r = await sendCall(who.id, { method: 'POST', path: '/api/v1/till/write-off', label: `Write-off: ${what}`, body: { productId: x.id, batchId: lot, quantity: qty, note, reason } })
    setBusy(false)
    if (r?.status === 'failed') return setError(r.note || 'The PC could not do this')
    onDone(`${DONE[reason]}: ${what}.${r ? '' : ' It sends when the PC is back.'}`)
  }

  return (
    <CounterShell title={x.name} sub={`${qtyText(x.stockQty)} ${x.unitAbbr} on hand`} onBack={onBack}>
      <form onSubmit={submit} autoComplete="off" className="flex flex-col gap-3">
        <div role="radiogroup" aria-label="Reason" className="grid grid-cols-3 gap-1.5">
          {REASONS.map((r) => (
            <button key={r.v} type="button" role="radio" aria-checked={reason === r.v} onClick={() => setReason(r.v)}
              className="min-h-14 rounded-[var(--radius-sm)] border border-border bg-muted px-2 text-[13px] font-semibold aria-checked:border-primary aria-checked:bg-primary aria-checked:text-primary-foreground">{r.label}</button>
          ))}
        </div>
        {lots.length > 1 && (
          <div role="radiogroup" aria-label="Lot" className="flex flex-col gap-1">
            {[{ id: 0, label: 'Oldest first', sub: '' }, ...lots.map((l) => ({ id: l.batchId, label: `${l.batchNo || `Lot ${l.batchId}`} · ${qtyText(l.qtyRemaining)} ${x.unitAbbr}`, sub: [l.expiryDate && `exp ${l.expiryDate}`, money(copy.symbol, l.price)].filter(Boolean).join(' · ') }))].map((o) => (
              <button key={o.id} type="button" role="radio" aria-checked={lot === o.id} onClick={() => setLot(o.id)}
                className="flex min-h-11 items-center justify-between gap-2 rounded-[var(--radius-sm)] border border-border px-3 text-left text-[13px] aria-checked:border-primary aria-checked:bg-primary aria-checked:text-primary-foreground">
                <b className="truncate">{o.label}</b><small className="shrink-0 opacity-75 tabular-nums">{o.sub}</small>
              </button>
            ))}
          </div>
        )}
        <input aria-label="Quantity" inputMode="decimal" value={qty} onChange={(e) => setQty(e.target.value.replace(/[^\d.]/g, ''))} className={cn(FIELD, 'h-14 w-full font-display text-2xl font-bold tabular-nums')} />
        {over && <p className="px-1 text-[13px] font-semibold text-warning">That's more than the {qtyText(shown)} shown. It still saves, for the PC to check.</p>}
        <input aria-label="Note" placeholder="What happened (optional)" value={note} onChange={(e) => setNote(e.target.value)} className={cn(FIELD, 'h-12 w-full')} />
        {error && <p role="alert" className="px-1 text-[13px] font-semibold text-danger">{error}</p>}
        <Button type="submit" size="lg" disabled={busy || !(num(qty) > 0)} className="h-12 text-[15px]">Take {qtyText(num(qty) || 0)} {x.unitAbbr} out of stock</Button>
      </form>
    </CounterShell>
  )
}
