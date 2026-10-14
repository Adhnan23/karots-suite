import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminMoneyRequest, AdminSupplierDetail, AdminSupplierPayRequest, ReceiptRef } from '@/api/types.gen'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { Seg } from '@/features/till/bits'
import { errMsg } from '@/features/till/api'
import { slipPath } from '@/features/till/print'
import { slipOut } from '@/features/till/SlipAsk'
import { dec, money, num, r2 } from '@/lib/money'
import { cn } from '@/lib/utils'
import { type Method, MoneySourcePicker } from '../MoneySource'

const amt = (v: string) => v.replace(/[^\d.]/g, '')

/** After an admin money move: say so, and its slip goes by Settings' print rule. */
export function moneyDone(msg: string, r: ReceiptRef) {
  if (!r.receiptId) return void toast.success(msg)
  void slipOut(slipPath.money(r.receiptId), msg)
}

function MethodSource({ method, setMethod, source, setSource, sym, into }: {
  method: Method; setMethod: (m: Method) => void; source: string; setSource: (s: string) => void; sym: string; into?: boolean
}) {
  const where = method === 'cash' ? (into ? 'Put the cash into' : 'Pay the cash from') : into ? 'Received into (optional)' : 'Paid from (optional)'
  return (
    <>
      <Seg label="Method" value={method} onChange={(m) => { setMethod(m); setSource('') }} options={[['cash', 'Cash'], ['card', 'Card'], ['online', 'Online']]} />
      {method === 'card' ? (
        <p className="text-[13px] text-muted-foreground">{into ? "A card refund from a supplier goes back to the owner's card — nothing lands in a shop account." : "A card payment to a supplier is the owner's card — nothing leaves a shop account."}</p>
      ) : (
        <Field label={where} hint={method === 'online' ? 'Pick the bank account so its balance stays right.' : undefined}>
          <MoneySourcePicker method={method} value={source} onChange={setSource} sym={sym} into={into} label={where} />
        </Field>
      )}
    </>
  )
}

/** Pay a supplier (karots-pos Pay Supplier): one amount settles oldest bills
 * first (or old debt first), or split it exactly across bills, old debt and an
 * advance. Anything over what's owed stays with them as credit. */
export function PaySheet({ s, sym, onClose }: { s: AdminSupplierDetail; sym: string; onClose: () => void }) {
  const qc = useQueryClient()
  const owed = Math.max(num(s.owed), 0)
  const old = num(s.oldDebt)
  const [amount, setAmount] = useState(owed > 0 ? dec(owed) : '')
  const [mode, setMode] = useState<'pay' | 'old'>('pay')
  const [exact, setExact] = useState(false)
  const [alloc, setAlloc] = useState<Record<number, string>>({})
  const [oldDebt, setOldDebt] = useState('')
  const [advance, setAdvance] = useState('')
  const [method, setMethod] = useState<Method>('cash')
  const [source, setSource] = useState('')
  const [reference, setReference] = useState('')
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const total = exact ? r2(Object.values(alloc).reduce((t, v) => t + num(v), 0) + num(oldDebt) + num(advance)) : num(amount)
  async function save() {
    setBusy(true)
    try {
      const body: AdminSupplierPayRequest = exact
        ? { amount: '', mode, allocations: Object.entries(alloc).map(([id, a]) => ({ purchaseId: Number(id), amount: a })), advance, oldDebt, method, reference, note, source }
        : { amount, mode, allocations: [], advance: '', oldDebt: '', method, reference, note, source }
      moneyDone(`Paid ${money(sym, total)} to ${s.name}`, await api<ReceiptRef>(`/admin/suppliers/${s.id}/payment`, { json: body }))
      void qc.invalidateQueries({ queryKey: ['admin'] })
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Pay ${s.name}`} submit={`↑ Pay ${total > 0 ? money(sym, total) : ''}`} busy={busy || !(total > 0)} onSubmit={save} wide>
      <p className="text-sm text-muted-foreground">Owed <b className="text-foreground tabular-nums">{money(sym, owed)}</b>{old > 0 && <> · old debt {money(sym, old)}</>}</p>
      {s.invoices.length > 0 && (
        <Seg label="How to split" value={exact ? 'exact' : 'one'} onChange={(v) => setExact(v === 'exact')} options={[['one', 'One amount'], ['exact', 'Split exactly']]} />
      )}
      {!exact ? (
        <>
          <Field label="Amount paid" hint={total > owed && owed >= 0 && total > 0 ? `${money(sym, total - owed)} beyond what's owed stays with them as an advance.` : undefined}>
            <div className="flex gap-1.5">
              <input autoFocus aria-label="Amount paid" inputMode="decimal" value={amount} onChange={(e) => setAmount(amt(e.target.value))} placeholder="0.00"
                className={cn(INPUT, 'h-[52px] min-w-0 flex-1 font-display text-2xl font-bold tabular-nums')} />
              {owed > 0 && <button type="button" onClick={() => setAmount(dec(owed))} className="rounded-[var(--radius-sm)] border border-border px-3 text-[13px] font-bold hover:bg-muted">All</button>}
            </div>
          </Field>
          {old > 0 && s.invoices.length > 0 && <Seg label="Pay first" value={mode} onChange={setMode} options={[['pay', 'Bills first'], ['old', 'Old debt first']]} />}
        </>
      ) : (
        <div className="flex flex-col gap-1.5 rounded-[var(--radius-sm)] bg-muted p-3 text-sm">
          {s.invoices.map((i) => (
            <label key={i.id} className="flex items-center gap-2">
              <span className="min-w-0 flex-1 truncate">{i.invoiceNo || `#${i.id}`} · {new Date(i.date).toLocaleDateString([], { day: 'numeric', month: 'short' })}
                <small className="block text-faint tabular-nums">owed {money(sym, i.balance)} of {money(sym, i.total)}</small></span>
              <input aria-label={`Pay on ${i.invoiceNo || `#${i.id}`}`} inputMode="decimal" placeholder={dec(num(i.balance))} value={alloc[i.id] ?? ''}
                onChange={(e) => setAlloc({ ...alloc, [i.id]: amt(e.target.value) })} className={cn(INPUT, 'h-10 w-28 text-right tabular-nums')} />
            </label>
          ))}
          {old > 0 && (
            <label className="flex items-center gap-2"><span className="flex-1">Old debt <small className="block text-faint tabular-nums">owed {money(sym, old)}</small></span>
              <input aria-label="Pay on old debt" inputMode="decimal" placeholder="0.00" value={oldDebt} onChange={(e) => setOldDebt(amt(e.target.value))} className={cn(INPUT, 'h-10 w-28 text-right tabular-nums')} /></label>
          )}
          <label className="flex items-center gap-2"><span className="flex-1">Advance <small className="block text-faint">sits as credit for their next delivery</small></span>
            <input aria-label="Advance" inputMode="decimal" placeholder="0.00" value={advance} onChange={(e) => setAdvance(amt(e.target.value))} className={cn(INPUT, 'h-10 w-28 text-right tabular-nums')} /></label>
        </div>
      )}
      <MethodSource method={method} setMethod={setMethod} source={source} setSource={setSource} sym={sym} />
      <div className="grid grid-cols-2 gap-3">
        <Field label="Reference"><input aria-label="Reference" value={reference} onChange={(e) => setReference(e.target.value)} className={INPUT} placeholder="Cheque / txn no" /></Field>
        <Field label="Note"><input aria-label="Note" value={note} onChange={(e) => setNote(e.target.value)} className={INPUT} placeholder="Optional" /></Field>
      </div>
    </FormSheet>
  )
}

/** Money a supplier hands back when they hold our credit (capped at it). */
export function RefundSheet({ s, sym, onClose }: { s: AdminSupplierDetail; sym: string; onClose: () => void }) {
  const qc = useQueryClient()
  const credit = Math.max(-num(s.owed), 0)
  const [amount, setAmount] = useState(dec(credit))
  const [method, setMethod] = useState<Method>('cash')
  const [source, setSource] = useState('')
  const [reference, setReference] = useState('')
  const [busy, setBusy] = useState(false)
  async function save() {
    setBusy(true)
    try {
      const body: AdminMoneyRequest = { amount, method, reference, note: '', source }
      moneyDone(`Received ${money(sym, amount)} back from ${s.name}`, await api<ReceiptRef>(`/admin/suppliers/${s.id}/refund`, { json: body }))
      void qc.invalidateQueries({ queryKey: ['admin'] })
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Refund from ${s.name}`} submit={`↓ Record ${money(sym, amount)}`} busy={busy || !(num(amount) > 0)} onSubmit={save}>
      <p className="text-sm text-muted-foreground">They owe you <b className="text-success tabular-nums">{money(sym, credit)}</b>. Record what they hand back — or leave it as credit for their next delivery.</p>
      <Field label="Amount received">
        <input autoFocus aria-label="Amount received" inputMode="decimal" value={amount} onChange={(e) => setAmount(amt(e.target.value))} className={cn(INPUT, 'h-[52px] font-display text-2xl font-bold tabular-nums')} />
      </Field>
      <MethodSource method={method} setMethod={setMethod} source={source} setSource={setSource} sym={sym} into />
      <Field label="Reference"><input aria-label="Reference" value={reference} onChange={(e) => setReference(e.target.value)} className={INPUT} placeholder="Cheque / txn no" /></Field>
    </FormSheet>
  )
}
