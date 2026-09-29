import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { ReceiptRef, SupplierPayRequest, SupplierRefundRequest, SupplierView } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Seg } from '@/features/till/bits'
import { configQuery, drawerQuery, errMsg, lockersOf } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { dec, money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { afterSlip, moneySlip } from '../slip'
import { SourcePicker } from '../ui'

type Method = 'cash' | 'card' | 'online'
const LABEL = 'flex flex-col gap-1.5 text-[13px] font-semibold text-muted-foreground'

/** Money to or from a supplier: where it comes from (drawer / locker / outside the shop) matters. */
export function PayForm({ s }: { s: SupplierView }) {
  const cfg = useSuspenseQuery(configQuery).data
  const drawer = useQuery(drawerQuery).data
  const qc = useQueryClient()
  const sym = cfg.symbol
  const owed = num(s.owed)
  const [refund, setRefund] = useState(false)
  const [amount, setAmount] = useState(owed > 0 ? dec(owed) : '')
  const [mode, setMode] = useState<'pay' | 'old'>('pay')
  const [method, setMethod] = useState<Method>('cash')
  const [source, setSource] = useState('')
  const [ref, setRef] = useState('')
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const cashLockers = (drawer?.lockers ?? []).filter((l) => l.kind !== 'card' && l.kind !== 'bank')
  // A card machine only takes money in: paying a supplier by card is the owner's
  // card (outside the shop). Online may leave a shop bank account (karots-pos).
  const accounts = method === 'online' ? lockersOf(drawer, 'bank') : []
  const a = num(amount)

  async function submit(e: { preventDefault(): void }) {
    e.preventDefault()
    if (!(a > 0)) return void toast.error('Enter the amount')
    setBusy(true)
    try {
      const base = { amount, method, reference: ref, note, lockerId: num(source), deferDrawerKick: !cfg.askToPrint }
      const r = refund
        ? await api<ReceiptRef>(`/till/suppliers/${s.id}/refund`, { json: base satisfies SupplierRefundRequest })
        : await api<ReceiptRef>(`/till/suppliers/${s.id}/pay`, { json: { ...base, mode } satisfies SupplierPayRequest })
      const msg = refund ? `Received ${money(sym, a)} back from ${s.name}` : `Paid ${money(sym, a)} to ${s.name}`
      afterSlip(cfg, msg, moneySlip(r.receiptId), method === 'cash' && source === '')
      setAmount('')
      void qc.invalidateQueries({ queryKey: ['till'] })
    } catch (err) {
      toast.error(errMsg(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={submit} autoComplete="off" className="flex min-h-0 flex-1 flex-col gap-4 overflow-auto px-5 pb-5">
      {owed < 0 && (
        <Seg label="Direction" value={refund ? 'in' : 'out'} onChange={(v) => { setRefund(v === 'in'); setAmount(v === 'in' ? dec(-owed) : '') }}
          options={[['out', 'We pay them'], ['in', 'They pay us back']]} />
      )}
      {!refund && (s.invoices.length > 0 || num(s.oldDebt) > 0) && (
        <div className="rounded-[var(--radius-sm)] bg-muted p-3 text-[13px]">
          <div className="mb-1.5 flex items-center justify-between">
            <b>What's owed</b>
            {s.invoices.length > 0 && num(s.oldDebt) > 0 && (
              <Seg label="Pay first" value={mode} onChange={setMode} options={[['pay', 'Invoices first'], ['old', 'Old debt first']]} />
            )}
          </div>
          <ul className="flex flex-col gap-0.5 tabular-nums">
            {num(s.oldDebt) > 0 && <li className="flex justify-between"><span>Old debt</span><b>{money(sym, s.oldDebt)}</b></li>}
            {s.invoices.map((i) => (
              <li key={i.id} className="flex justify-between gap-3">
                <span>{i.invoiceNo || `#${i.id}`} · {new Date(i.date).toLocaleDateString([], { day: 'numeric', month: 'short' })}{i.dueDate && ` · due ${i.dueDate}`}</span>
                <b>{money(sym, i.balance)}</b>
              </li>
            ))}
          </ul>
        </div>
      )}
      <label className={LABEL}>
        {refund ? 'Amount received' : 'Amount paid'}
        <div className="flex gap-1.5">
          <input aria-label="Amount" inputMode="decimal" placeholder={money(sym, 0)} value={amount} onChange={(e) => setAmount(e.target.value.replace(/[^\d.]/g, ''))}
            className={cn(FIELD, 'h-14 min-w-0 flex-1 font-display text-2xl font-bold tabular-nums')} />
          {!refund && owed > 0 && (
            <button type="button" onClick={() => setAmount(dec(owed))} className="rounded-[var(--radius-sm)] border border-border px-3 text-[13px] font-bold text-foreground hover:bg-muted">All {money(sym, owed)}</button>
          )}
        </div>
        {!refund && a > owed && owed >= 0 && a > 0 && <small className="font-normal">{money(sym, a - Math.max(owed, 0))} beyond what's owed stays with them as an advance.</small>}
      </label>
      <Seg label="Method" value={method} onChange={(m) => { setMethod(m); setSource('') }} options={[['cash', 'Cash'], ['card', 'Card'], ['online', 'Online']]} />
      {method === 'cash' ? (
        <div className={LABEL}>{refund ? 'Into' : 'From'}<SourcePicker value={source} onChange={setSource} lockers={cashLockers} sym={sym} /></div>
      ) : (
        <div className="grid grid-cols-2 gap-2">
          <select aria-label="Account" value={source} onChange={(e) => setSource(e.target.value)} disabled={!accounts.length} className={cn(FIELD, 'appearance-auto disabled:opacity-60')}>
            <option value="">{refund ? 'Not into a shop account' : 'Not from a shop account'}</option>
            {accounts.map((l) => <option key={l.id} value={l.id}>{l.name}</option>)}
          </select>
          <input aria-label="Reference" placeholder="Reference (optional)" value={ref} onChange={(e) => setRef(e.target.value)} className={FIELD} />
        </div>
      )}
      <input aria-label="Note" placeholder="Note (optional)" value={note} onChange={(e) => setNote(e.target.value)} className={FIELD} />
      <Button type="submit" size="lg" disabled={busy || !(a > 0)} className="mt-auto h-12 text-[15px]">
        {refund ? `Take ${a > 0 ? money(sym, a) : ''} back` : `Pay ${a > 0 ? money(sym, a) : ''}`}
      </Button>
    </form>
  )
}
