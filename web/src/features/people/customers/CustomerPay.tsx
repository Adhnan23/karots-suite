import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminCustomerDetail, CreditPaymentResult, CustomerPayRequest } from '@/api/types.gen'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { type Method, MoneySourcePicker } from '@/features/purchasing/MoneySource'
import { Seg } from '@/features/till/bits'
import { errMsg } from '@/features/till/api'
import { printDebtSlip } from '@/features/till/print'
import { dec, money, num } from '@/lib/money'
import { cn } from '@/lib/utils'

/** Collect a credit repayment (karots-pos Collect Payment): cash into a locker
 * or open till, card / online into its account. Over what's owed = an advance. */
export function PaySheet({ c, sym, onClose }: { c: AdminCustomerDetail; sym: string; onClose: () => void }) {
  const qc = useQueryClient()
  const owed = Math.max(num(c.owed), 0)
  const old = num(c.oldDebt)
  const [amount, setAmount] = useState(owed > 0 ? dec(owed) : '')
  const [method, setMethod] = useState<Method>('cash')
  const [source, setSource] = useState('')
  const [oldFirst, setOldFirst] = useState(false)
  const [reference, setReference] = useState('')
  const [busy, setBusy] = useState(false)
  const a = num(amount)
  const where = method === 'cash' ? 'Cash into' : method === 'card' ? 'Card account (optional)' : 'Bank account (optional)'
  async function save() {
    setBusy(true)
    try {
      const body: CustomerPayRequest = { amount, method, reference, note: '', source, oldDebtFirst: oldFirst }
      const r = await api<CreditPaymentResult>(`/admin/customers/${c.id}/payment`, { json: body })
      toast.success(`Collected ${money(sym, a)} · ${r.receiptNo}`, { duration: 10_000, action: { label: 'Print slip', onClick: () => void printDebtSlip(r.paymentId) } })
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
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Collect from ${c.name}`} submit={`↓ Collect ${a > 0 ? money(sym, a) : ''}`} busy={busy || !(a > 0)} onSubmit={save}>
      <p className="text-sm text-muted-foreground">Owes <b className="text-foreground tabular-nums">{money(sym, owed)}</b>{old > 0 && <> · old debt {money(sym, old)}</>}</p>
      <Field label="Amount received" hint={a > owed && a > 0 ? `${money(sym, a - owed)} over what's owed stays as their advance.` : undefined}>
        <div className="flex gap-1.5">
          <input autoFocus aria-label="Amount received" inputMode="decimal" value={amount} onChange={(e) => setAmount(e.target.value.replace(/[^\d.]/g, ''))} placeholder="0.00"
            className={cn(INPUT, 'h-[52px] min-w-0 flex-1 font-display text-2xl font-bold tabular-nums')} />
          {owed > 0 && <button type="button" onClick={() => setAmount(dec(owed))} className="rounded-[var(--radius-sm)] border border-border px-3 text-[13px] font-bold hover:bg-muted">All</button>}
        </div>
      </Field>
      {old > 0 && <Seg label="Clear first" value={oldFirst ? 'old' : 'new'} onChange={(v) => setOldFirst(v === 'old')} options={[['new', 'Recent bills first'], ['old', 'Old debt first']]} />}
      <Seg label="Method" value={method} onChange={(m) => { setMethod(m); setSource('') }} options={[['cash', 'Cash'], ['card', 'Card'], ['online', 'Online']]} />
      <Field label={where}><MoneySourcePicker method={method} value={source} onChange={setSource} sym={sym} into label={where} /></Field>
      <Field label="Reference"><input aria-label="Reference" value={reference} onChange={(e) => setReference(e.target.value)} className={INPUT} placeholder="Optional" /></Field>
    </FormSheet>
  )
}
