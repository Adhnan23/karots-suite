import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { ChequeClearRequest, ChequeDetail, ChequeFixRequest, ChequeReturnRequest } from '@/api/types.gen'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { MethodBadge } from '@/components/admin/MethodBadge'
import { Button } from '@/components/ui/button'
import { bankPlaces, moneySourcesQuery } from '@/features/purchasing/MoneySource'
import { errMsg } from '@/features/till/api'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { dueText } from './ChequesPage'
import { chequeBanksQuery, chequeQuery, today } from './queries'

const day = (d: string) => new Date(`${d}T00:00`).toLocaleDateString([], { day: 'numeric', month: 'short', year: 'numeric' })

/** Where a cheque came from, as a link to that record. */
function SourceLink({ d }: { d: ChequeDetail }) {
  const s = d.source
  if (!s.kind) return <span className="text-faint">—</span>
  if (s.kind === 'sale') return <Link to="/admin/sales" search={{ q: s.label.replace(/^Sale /, '') }} className="font-semibold text-brand underline-offset-2 hover:underline">{s.label}</Link>
  if (s.kind === 'customer_payment' && d.customerId) return <Link to="/admin/customers/$id" params={{ id: String(d.customerId) }} className="font-semibold text-brand underline-offset-2 hover:underline">{s.label}</Link>
  if (s.kind === 'supplier_payment' && d.supplierId) return <Link to="/admin/suppliers/$id" params={{ id: String(d.supplierId) }} className="font-semibold text-brand underline-offset-2 hover:underline">{s.label}</Link>
  if (s.kind === 'expense') return <Link to="/admin/expenses" className="font-semibold text-brand underline-offset-2 hover:underline">{s.label}</Link>
  return <span>{s.label}</span>
}

/** One cheque: its details, where it came from, its history — and, while it is
 * Pending, Cleared… / Bounced / Cancelled / Fix details. Final once marked. */
export function ChequeSheet({ id, sym, onClose }: { id: number; sym: string; onClose: () => void }) {
  const qc = useQueryClient()
  const d = useQuery(chequeQuery(id)).data
  const banks = bankPlaces(useQuery(moneySourcesQuery).data)
  const names = useQuery(chequeBanksQuery).data ?? []
  const [mode, setMode] = useState<'view' | 'clear' | 'fix'>('view')
  const [clearOn, setClearOn] = useState(today())
  const [into, setInto] = useState('')
  const [fix, setFix] = useState<ChequeFixRequest | null>(null)
  const [busy, setBusy] = useState(false)
  if (!d) return null
  const received = d.direction === 'in'
  const pending = d.status === 'pending'
  const into0 = into || String(d.lastBankId || banks[0]?.id || '')
  const done = (msg: string) => {
    toast.success(msg)
    void qc.invalidateQueries({ queryKey: ['admin'] })
    onClose()
  }
  async function run(fn: () => Promise<unknown>, msg: string) {
    if (busy) return
    setBusy(true)
    try {
      await fn()
      done(msg)
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  const ret = (status: 'bounced' | 'cancelled') =>
    run(() => api(`/admin/cheques/${id}/return`, { json: { status } satisfies ChequeReturnRequest }), `Cheque ${d.number} marked ${status} — ${money(sym, d.amount)} is owed again`)

  if (mode === 'clear')
    return (
      <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Cleared — cheque ${d.number}`} submit={`Mark cleared · ${money(sym, d.amount)}`}
        busy={busy || (received && !into0)} onSubmit={() => void run(() => api(`/admin/cheques/${id}/clear`, { json: { date: clearOn, lockerId: received ? Number(into0) : 0 } satisfies ChequeClearRequest }), `Cheque ${d.number} cleared`)}>
        {received ? (
          <Field label="Went into">
            <select aria-label="Went into" value={into0} onChange={(e) => setInto(e.target.value)} className={cn(INPUT, 'appearance-auto')}>
              {banks.map((b) => <option key={b.id} value={b.id}>{b.name}</option>)}
            </select>
          </Field>
        ) : <p className="text-sm text-muted-foreground">{money(sym, d.amount)} leaves <b className="text-foreground">{d.lockerName}</b> on this day.</p>}
        <Field label="Cleared on" hint="Today, or back as far as the cheque date.">
          <input type="date" aria-label="Cleared on" value={clearOn} min={d.chequeDate} max={today()} onChange={(e) => setClearOn(e.target.value)} className={INPUT} />
        </Field>
      </FormSheet>
    )

  if (mode === 'fix' && fix)
    return (
      <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Fix cheque ${d.number}`} submit="Save" busy={busy || !fix.number.trim() || !fix.date}
        onSubmit={() => void run(() => api(`/admin/cheques/${id}`, { method: 'PUT', json: fix }), 'Cheque details saved')}>
        <p className="text-sm text-muted-foreground">The amount and who it's {received ? 'from' : 'for'} can't change — cancel it and take a new one instead.</p>
        {received ? (
          <Field label="Bank">
            <input aria-label="Bank" list="cheque-fix-banks" value={fix.bank} onChange={(e) => setFix({ ...fix, bank: e.target.value })} className={INPUT} />
            <datalist id="cheque-fix-banks">{names.map((b) => <option key={b} value={b} />)}</datalist>
          </Field>
        ) : (
          <Field label="Drawn on">
            <select aria-label="Drawn on" value={fix.lockerId} onChange={(e) => setFix({ ...fix, lockerId: Number(e.target.value) })} className={cn(INPUT, 'appearance-auto')}>
              {banks.map((b) => <option key={b.id} value={b.id}>{b.name}</option>)}
            </select>
          </Field>
        )}
        <div className="grid grid-cols-2 gap-3">
          <Field label="Cheque no."><input aria-label="Cheque number" value={fix.number} onChange={(e) => setFix({ ...fix, number: e.target.value })} className={INPUT} /></Field>
          <Field label="Cheque date"><input type="date" aria-label="Cheque date" value={fix.date} onChange={(e) => setFix({ ...fix, date: e.target.value })} className={INPUT} /></Field>
        </div>
        <Field label="Note"><input aria-label="Note" value={fix.note} onChange={(e) => setFix({ ...fix, note: e.target.value })} className={INPUT} placeholder="Optional" /></Field>
      </FormSheet>
    )

  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Cheque ${d.number}`} submit={pending ? 'Mark cleared…' : 'Close'} busy={busy}
      onSubmit={() => (pending ? setMode('clear') : onClose())}
      extra={pending && (
        <div className="flex flex-wrap gap-1.5">
          <ConfirmButton label="Mark bounced" title={`Cheque ${d.number} bounced?`} confirm="Mark bounced" onConfirm={() => ret('bounced')}
            body={`${money(sym, d.amount)} goes back as owed by ${d.who || 'them'}. This can't be undone.`} className="h-10 px-3" />
          <ConfirmButton plain label="Mark cancelled" title={`Cancel cheque ${d.number}?`} confirm="Mark cancelled" onConfirm={() => ret('cancelled')}
            body={`Called off by agreement: ${money(sym, d.amount)} goes back as owed. This can't be undone.`} className="h-10 px-3" />
          <Button type="button" variant="ghost" className="h-10 px-3" onClick={() => { setFix({ bank: d.bank, lockerId: d.lockerId, number: d.number, date: d.chequeDate, note: d.note }); setMode('fix') }}>Fix details</Button>
        </div>
      )}>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <small className="text-muted-foreground">{received ? 'From' : 'To'}</small>
          <b className="block truncate font-display text-lg">{d.who || '—'}</b>
        </div>
        <b className="font-display text-2xl tabular-nums text-cheque">{money(sym, d.amount)}</b>
      </div>
      <dl className="grid grid-cols-2 gap-x-3 gap-y-2 rounded-[var(--radius-sm)] bg-muted p-3 text-[13px]">
        <dt className="text-muted-foreground">Status</dt>
        <dd><MethodBadge method="cheque" label={d.status.charAt(0).toUpperCase() + d.status.slice(1)} status={pending ? undefined : d.status} />{d.statusDate && <small className="ml-1.5 text-faint">{day(d.statusDate)}</small>}</dd>
        <dt className="text-muted-foreground">Cheque date</dt>
        <dd className="tabular-nums">{day(d.chequeDate)}{pending && <small className="ml-1.5 text-faint">{dueText(d.days)}</small>}</dd>
        <dt className="text-muted-foreground">{received ? 'Bank' : 'Drawn on'}</dt>
        <dd>{received ? d.bank : d.lockerName}</dd>
        {received && d.lockerName && <><dt className="text-muted-foreground">Cleared into</dt><dd>{d.lockerName}</dd></>}
        <dt className="text-muted-foreground">Number</dt>
        <dd className="tabular-nums">{d.number}</dd>
        <dt className="text-muted-foreground">Came from</dt>
        <dd className="min-w-0 truncate"><SourceLink d={d} /></dd>
        {d.note && <><dt className="text-muted-foreground">Note</dt><dd>{d.note}</dd></>}
      </dl>
      {d.history.length > 0 && (
        <section aria-label="History">
          <h3 className="mb-1.5 text-[13px] font-bold">History</h3>
          <ul className="flex flex-col gap-1 text-[13px]">
            {d.history.map((h, i) => (
              <li key={i} className="flex flex-wrap justify-between gap-x-3"><span>{h.detail}</span>
                <small className="text-faint">{h.user || 'Someone'} · {new Date(h.at).toLocaleString([], { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })}</small></li>
            ))}
          </ul>
        </section>
      )}
    </FormSheet>
  )
}
