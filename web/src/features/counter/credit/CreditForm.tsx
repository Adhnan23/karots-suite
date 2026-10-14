import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { Banknote, CreditCard, Smartphone } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdvanceRefundRequest, CreditPaymentRequest, CreditPaymentResult, ReceiptRef, TillCustomer } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Seg } from '@/features/till/bits'
import { configQuery, drawerQuery, errMsg, lockersOf } from '@/features/till/api'
import { slipPath } from '@/features/till/print'
import { FIELD } from '@/features/till/ui'
import { dec, money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { afterSlip, moneySlip } from '../slip'

type Mode = 'pay' | 'refund'
const LABEL = 'flex flex-col gap-1.5 text-[13px] font-semibold text-muted-foreground'

/** A plugin may open it pre-filled (amount, note, mode) and hear the result (onDone),
 * e.g. a deposit taken against a job. */
export type CreditFormOpts = { amount?: string; note?: string; mode?: Mode; onDone?: (r: { mode: Mode; paymentId: number; amount: string }) => void }

export function CreditForm({ c, opts }: { c: TillCustomer; opts?: CreditFormOpts }) {
  const cfg = useSuspenseQuery(configQuery).data
  const drawer = useQuery(drawerQuery).data
  const qc = useQueryClient()
  const sym = cfg.symbol
  const bal = num(c.outstanding)
  const advance = bal < 0 ? -bal : 0
  const [mode, setMode] = useState<Mode>(opts?.mode ?? 'pay')
  const [amount, setAmount] = useState(opts?.amount ?? (bal > 0 ? dec(bal) : ''))
  const [method, setMethod] = useState('cash')
  const [ref, setRef] = useState('')
  const [note, setNote] = useState(opts?.note ?? '')
  const [busy, setBusy] = useState(false)
  const accounts = method === 'card' ? lockersOf(drawer, 'card') : method === 'online' ? lockersOf(drawer, 'bank') : []
  const [account, setAccount] = useState('')
  const methods = [
    { v: 'cash', label: 'Cash', Icon: Banknote },
    ...(lockersOf(drawer, 'card').length ? [{ v: 'card', label: 'Card', Icon: CreditCard }] : []),
    ...(lockersOf(drawer, 'bank').length ? [{ v: 'online', label: 'Online', Icon: Smartphone }] : []),
  ]
  const a = num(amount)
  const after = mode === 'pay' ? bal - a : bal + a

  async function submit(e: { preventDefault(): void }) {
    e.preventDefault()
    if (!(a > 0)) return void toast.error('Enter the amount')
    setBusy(true)
    try {
      if (mode === 'pay') {
        const body: CreditPaymentRequest = { note, amount, method, accountId: num(account || accounts[0]?.id), reference: ref, deferDrawerKick: !cfg.askToPrint }
        const r = await api<CreditPaymentResult>(`/till/customers/${c.id}/payment`, { json: body })
        afterSlip(cfg, `${money(sym, a)} collected from ${c.name} · ${r.receiptNo}`, slipPath.debt(r.paymentId), method === 'cash')
        opts?.onDone?.({ mode, paymentId: r.paymentId, amount })
      } else {
        const body: AdvanceRefundRequest = { amount, deferDrawerKick: !cfg.askToPrint }
        const r = await api<ReceiptRef>(`/till/customers/${c.id}/refund`, { json: body })
        afterSlip(cfg, `Give ${c.name} ${money(sym, a)} — advance refunded`, moneySlip(r.receiptId), true)
        opts?.onDone?.({ mode, paymentId: 0, amount })
      }
      void qc.invalidateQueries({ queryKey: ['till'] })
    } catch (err) {
      toast.error(errMsg(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={submit} autoComplete="off" className="flex flex-col gap-5 overflow-auto p-5">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="font-display text-xl font-bold">{c.name}</h2>
          <p className="text-[13px] text-muted-foreground tabular-nums">{c.phone || 'no phone'}{num(c.creditLimit) > 0 && ` · limit ${money(sym, c.creditLimit)}`}</p>
        </div>
        {advance > 0 && !opts?.mode && (
          <Seg label="Action" value={mode} onChange={(m) => { setMode(m); setAmount(m === 'refund' ? dec(advance) : '') }}
            options={[['pay', 'Take payment'], ['refund', 'Refund advance']]} />
        )}
      </div>
      <div className="grid grid-cols-2 gap-2">
        <Stat label={bal >= 0 ? 'Owes now' : 'Advance now'} value={money(sym, Math.abs(bal))} tone={bal > 0 ? 'text-warning' : 'text-success'} />
        <Stat label={after > 0.004 ? 'Will owe' : after < -0.004 ? 'Advance after' : 'After'} value={a > 0 ? money(sym, Math.abs(after)) : '—'}
          tone={a > 0 && Math.abs(after) < 0.005 ? 'text-success' : 'text-foreground'} />
      </div>
      <label className={LABEL}>
        {mode === 'pay' ? 'Amount received' : 'Amount to hand back'}
        <div className="flex gap-1.5">
          <input aria-label="Amount" inputMode="decimal" placeholder={money(sym, 0)} value={amount} onChange={(e) => setAmount(e.target.value.replace(/[^\d.]/g, ''))}
            className={cn(FIELD, 'h-14 min-w-0 flex-1 font-display text-2xl font-bold tabular-nums')} />
          {mode === 'pay' && bal > 0 && (
            <button type="button" onClick={() => setAmount(dec(bal))} className="rounded-[var(--radius-sm)] border border-border px-3 text-[13px] font-bold text-foreground hover:bg-muted">Full {money(sym, bal)}</button>
          )}
        </div>
      </label>
      {mode === 'pay' && methods.length > 1 && (
        <div role="radiogroup" aria-label="Paid by" className="grid grid-cols-3 gap-1.5">
          {methods.map(({ v, label, Icon }) => (
            <button key={v} type="button" role="radio" aria-checked={method === v} onClick={() => { setMethod(v); setAccount('') }}
              className="flex flex-col items-center gap-1 rounded-[var(--radius-sm)] border border-border bg-muted py-2 text-[12px] font-bold aria-checked:border-brand aria-checked:bg-brand aria-checked:text-brand-foreground">
              <Icon size={18} />{label}
            </button>
          ))}
        </div>
      )}
      {accounts.length > 1 && (
        <select aria-label="Account" value={account || String(accounts[0].id)} onChange={(e) => setAccount(e.target.value)} className={cn(FIELD, 'appearance-auto')}>
          {accounts.map((l) => <option key={l.id} value={l.id}>{l.name}</option>)}
        </select>
      )}
      {mode === 'pay' && method !== 'cash' && (
        <input aria-label="Reference" placeholder="Reference (optional)" value={ref} onChange={(e) => setRef(e.target.value)} className={FIELD} />
      )}
      {mode === 'pay' && <input aria-label="Note" placeholder="Note (optional)" value={note} onChange={(e) => setNote(e.target.value)} className={FIELD} />}
      <Button type="submit" size="lg" disabled={busy || !(a > 0)} className="h-12 text-[15px]">
        {mode === 'pay' ? `Collect ${a > 0 ? money(sym, a) : ''}` : `Refund ${a > 0 ? money(sym, a) : ''} from the drawer`}
      </Button>
    </form>
  )
}

function Stat({ label, value, tone }: { label: string; value: string; tone: string }) {
  return (
    <div className="rounded-[var(--radius-sm)] bg-muted px-3 py-2.5">
      <small className="text-muted-foreground">{label}</small>
      <b className={cn('block font-display text-2xl tabular-nums', tone)}>{value}</b>
    </div>
  )
}
