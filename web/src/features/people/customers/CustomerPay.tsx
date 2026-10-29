import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminCustomerDetail, CreditPaymentResult, CustomerPayRequest, CustomerRefundRequest, ReceiptRef, TenderMethod } from '@/api/types.gen'
import { blankCheque, ChequeLines, chequesReady, chequesTotal, toChequeLines, type ChequeDraftLine } from '@/components/admin/ChequeLines'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { type Method, MoneySourcePicker, moneySourcesQuery, sourcesFor } from '@/features/purchasing/MoneySource'
import { moneyDone } from '@/features/purchasing/suppliers/SupplierMoney'
import { Seg } from '@/features/till/bits'
import { errMsg } from '@/features/till/api'
import { slipPath } from '@/features/till/print'
import { slipOut } from '@/features/till/SlipAsk'
import { dec, money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { slots } from '@/plugins/slots'

const tenderMethodsQuery = { queryKey: ['admin', 'tender-methods'], queryFn: () => api<TenderMethod[]>('/admin/tender-methods') }

/** Collect a credit repayment: cash into a locker
 * or open till, card / online into its account — Card / Online only show when
 * the shop has that kind of account. Over what's owed = an advance. */
export function PaySheet({ c, sym, onClose }: { c: AdminCustomerDetail; sym: string; onClose: () => void }) {
  const qc = useQueryClient()
  const sources = useQuery(moneySourcesQuery).data ?? []
  const plugins = useQuery(tenderMethodsQuery).data ?? []
  const methods: [string, string][] = [
    ...([['cash', 'Cash'], ['card', 'Card'], ['online', 'Online']] as [Method, string][]).filter(([m]) => m === 'cash' || sourcesFor(sources, m).length > 0),
    ['cheque', 'Cheque'],
    ...plugins.map((t): [string, string] => [t.value, t.label]),
  ]
  const owed = Math.max(num(c.owed), 0)
  const old = num(c.oldDebt)
  const [amount, setAmount] = useState(owed > 0 ? dec(owed) : '')
  const [method, setMethod] = useState('cash')
  const [meta, setMeta] = useState<Record<string, string>>({}) // a plugin tender's detail
  const core = method === 'cash' || method === 'card' || method === 'online'
  const [cheques, setCheques] = useState<ChequeDraftLine[]>([])
  const PluginDetail = slots.tillTenders.find((t) => t.value === method)?.Detail
  const [source, setSource] = useState('')
  const [oldFirst, setOldFirst] = useState(false)
  const [reference, setReference] = useState('')
  const [busy, setBusy] = useState(false)
  const a = num(amount)
  const byCheque = method === 'cheque'
  const chequesOk = !byCheque || (chequesReady(cheques, 'in') && chequesTotal(cheques) === a)
  const where = method === 'cash' ? 'Cash into' : method === 'card' ? 'Card account' : 'Bank account'
  async function save() {
    setBusy(true)
    try {
      const body: CustomerPayRequest = { amount, method, reference, note: '', source: core ? source : '', oldDebtFirst: oldFirst, meta: core || byCheque ? undefined : meta,
        cheques: byCheque ? toChequeLines(cheques) : undefined }
      const r = await api<CreditPaymentResult>(`/admin/customers/${c.id}/payment`, { json: body })
      void slipOut(slipPath.debt(r.paymentId), `Collected ${money(sym, a)} · ${r.receiptNo}`)
      void qc.invalidateQueries({ queryKey: ['admin'] })
      void qc.invalidateQueries({ queryKey: ['till'] })
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Collect from ${c.name}`} submit={`↓ Collect ${a > 0 ? money(sym, a) : ''}`} busy={busy || !(a > 0) || !chequesOk} onSubmit={save}>
      <p className="text-sm text-muted-foreground">Owes <b className="text-foreground tabular-nums">{money(sym, owed)}</b>{old > 0 && <> · old debt {money(sym, old)}</>}</p>
      <Field label="Amount received" hint={a > owed && a > 0 ? `${money(sym, a - owed)} over what's owed stays as their advance.` : undefined}>
        <div className="flex gap-1.5">
          <input autoFocus aria-label="Amount received" inputMode="decimal" value={amount} onChange={(e) => setAmount(e.target.value.replace(/[^\d.]/g, ''))} placeholder="0.00"
            className={cn(INPUT, 'h-[52px] min-w-0 flex-1 font-display text-2xl font-bold tabular-nums')} />
          {owed > 0 && <button type="button" onClick={() => setAmount(dec(owed))} className="rounded-[var(--radius-sm)] border border-border px-3 text-[13px] font-bold hover:bg-muted">All</button>}
        </div>
      </Field>
      {old > 0 && <Seg label="Clear first" value={oldFirst ? 'old' : 'new'} onChange={(v) => setOldFirst(v === 'old')} options={[['new', 'Recent bills first'], ['old', 'Old debt first']]} />}
      {methods.length > 1 && <Seg label="Method" value={method} onChange={(m) => { setMethod(m); setMeta({}); if (m === 'cheque' && !cheques.length) setCheques([blankCheque(amount)]); setSource(m === 'cash' || !['card', 'online'].includes(m) ? '' : sourcesFor(sources, m as Method)[0]?.value ?? '') }} options={methods} />}
      {byCheque ? <ChequeLines dir="in" lines={cheques} onChange={setCheques} sym={sym} want={a} />
        : core ? <Field label={where}><MoneySourcePicker method={method as Method} value={source} onChange={setSource} sym={sym} into label={where} required /></Field>
        : PluginDetail && <PluginDetail meta={meta} setMeta={setMeta} />}
      <Field label="Reference"><input aria-label="Reference" value={reference} onChange={(e) => setReference(e.target.value)} className={INPUT} placeholder="Optional" /></Field>
    </FormSheet>
  )
}

/** Hand a customer's advance back in cash from a cash locker or open till, with
 * a refund receipt (the counter's Refund advance, from the back office). */
export function RefundSheet({ c, sym, onClose }: { c: AdminCustomerDetail; sym: string; onClose: () => void }) {
  const qc = useQueryClient()
  const advance = Math.max(-num(c.owed), 0)
  const [amount, setAmount] = useState(dec(advance))
  const [source, setSource] = useState('')
  const [busy, setBusy] = useState(false)
  const a = num(amount)
  async function save() {
    setBusy(true)
    try {
      const body: CustomerRefundRequest = { amount, source }
      moneyDone(`Give ${c.name} ${money(sym, a)} — advance refunded`, await api<ReceiptRef>(`/admin/customers/${c.id}/refund`, { json: body }))
      void qc.invalidateQueries({ queryKey: ['admin'] })
      void qc.invalidateQueries({ queryKey: ['till'] })
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Refund ${c.name}`} submit={`↑ Refund ${a > 0 ? money(sym, a) : ''}`} busy={busy || !(a > 0) || a > advance + 0.004 || !source} onSubmit={save}>
      <p className="text-sm text-muted-foreground">Holds an advance of <b className="text-foreground tabular-nums">{money(sym, advance)}</b></p>
      <Field label="Amount to hand back" hint={a > advance + 0.004 ? `No more than the ${money(sym, advance)} they hold.` : undefined}>
        <input autoFocus aria-label="Amount to hand back" inputMode="decimal" value={amount} onChange={(e) => setAmount(e.target.value.replace(/[^\d.]/g, ''))} placeholder="0.00"
          className={cn(INPUT, 'h-[52px] font-display text-2xl font-bold tabular-nums')} />
      </Field>
      <Field label="Cash from"><MoneySourcePicker method="cash" value={source} onChange={setSource} sym={sym} label="Cash from" /></Field>
    </FormSheet>
  )
}
