import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminSupplierPayRequest, LossRow, ProductLots, RecoveryRequest, SupplierPayRequest } from '@/api/types.gen'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { suppliersQuery } from '@/features/counter/queries'
import { Seg } from '@/features/till/bits'
import { errMsg } from '@/features/till/api'
import { BatchChoice } from '@/features/admin/intake/BatchChoice'
import { matchBatch } from '@/features/admin/stock/batches'
import { MoneySourcePicker } from '@/features/purchasing/MoneySource'
import { dec, money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'

/** What a supplier made good on one loss: replaced it (back in stock, asked
 * like intake — which batch, expiry, a new price, any extra they charged now
 * it costs more), credited us any amount (full or part), or nothing.
 * supplierId pre-picks the supplier (their delivery is being received);
 * base '/till' is the counter's Suppliers tab (an extra paid now leaves the drawer). */
export function RecoverSheet({ loss, sym, supplierId = 0, base = '/admin', onClose }: {
  loss: LossRow; sym: string; supplierId?: number; base?: '/admin' | '/till'; onClose: () => void
}) {
  const qc = useQueryClient()
  const sups = useQuery(suppliersQuery('')).data ?? []
  const lotsData = useQuery({ queryKey: [base.slice(1), 'lots', loss.productId], queryFn: () => api<ProductLots>(`${base}/stock/lots/${loss.productId}`), enabled: loss.productId > 0 }).data
  const lots = lotsData?.lots ?? []
  const productPrice = lotsData?.sellingPrice ?? ''
  const [outcome, setOutcome] = useState<'replacement' | 'paid' | 'written_off'>('replacement')
  const [supplier, setSupplier] = useState(supplierId)
  const [amount, setAmount] = useState('')
  const [note, setNote] = useState('')
  const [extra, setExtra] = useState('')
  const [selling, setSelling] = useState('')
  const [expiry, setExpiry] = useState('')
  const [picked, setPicked] = useState<number | null>(null)
  const [payNow, setPayNow] = useState(false)
  const [source, setSource] = useState('')
  const [busy, setBusy] = useState(false)
  const lostUnit = num(loss.qty) > 0 ? num(loss.loss) / num(loss.qty) : 0
  const unitCost = lostUnit + num(extra)
  const auto = matchBatch(lots, { cost: dec(unitCost), selling: selling || productPrice, expiry }, productPrice)
  const batchId = picked ?? auto
  const needSupplier = outcome === 'paid' || (outcome === 'replacement' && num(extra) > 0)
  async function save() {
    setBusy(true)
    try {
      const replace = outcome === 'replacement'
      const body: RecoveryRequest = { sourceType: loss.kind, sourceId: loss.sourceId, outcome, supplierId: needSupplier ? supplier : 0, amount, note,
        batchId: replace ? batchId : 0, expiry: replace && !batchId ? expiry : '', extraPerUnit: replace ? extra : '', sellingPrice: replace && num(selling) !== num(productPrice) ? selling : '' }
      await api(`${base}/recovery`, { json: body })
      const extraTotal = dec(num(extra) * num(loss.qty))
      if (replace && payNow && num(extraTotal) > 0) {
        // The extra goes on what we owe them; paying it now takes it straight off again.
        const note = `extra for replacing ${loss.product}`
        try {
          if (base === '/till') await api(`/till/suppliers/${supplier}/pay`, { json: { amount: extraTotal, mode: 'pay', method: 'cash', reference: '', note, lockerId: 0, deferDrawerKick: false } satisfies SupplierPayRequest })
          else await api(`/admin/suppliers/${supplier}/payment`, { json: { amount: extraTotal, mode: 'pay', allocations: [], advance: '', oldDebt: '', method: 'cash', reference: '', note, source } satisfies AdminSupplierPayRequest })
        } catch (e) {
          // The replacement is booked; only the payment didn't go — the extra stays owed.
          toast.warning(`Replacement recorded, but the extra wasn't paid (${errMsg(e)}) — it stays on what you owe them.`)
          void qc.invalidateQueries({ queryKey: [base.slice(1)] })
          return onClose()
        }
      }
      toast.success('Recovery recorded')
      void qc.invalidateQueries({ queryKey: ['admin'] })
      void qc.invalidateQueries({ queryKey: ['till'] })
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  const clean = (v: string) => v.replace(/[^\d.]/g, '')
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Recover: ${loss.product}`} submit="Record"
      busy={busy || (needSupplier && !supplier) || (payNow && base === '/admin' && num(extra) > 0 && !source)} onSubmit={save}>
      <p className="text-sm text-muted-foreground">Lost {qtyText(loss.qty)} worth {money(sym, loss.loss)} at cost ({money(sym, lostUnit)} each).</p>
      <Seg label="What the supplier did" value={outcome} onChange={setOutcome}
        options={[['replacement', 'Replaced it'], ['paid', 'Credited us'], ['written_off', 'Nothing']]} />
      {outcome === 'replacement' && (
        <>
          <p className="text-[13px] text-muted-foreground">{qtyText(loss.qty)} go back on the shelf.</p>
          <div className="grid grid-cols-2 gap-2">
            <Field label="Extra they asked, each" hint={`0 if free · cost ${money(sym, unitCost)} each`}>
              <input aria-label="Extra per unit" inputMode="decimal" value={extra} onChange={(e) => setExtra(clean(e.target.value))} placeholder="0.00" className={INPUT} />
            </Field>
            <Field label="Selling price">
              <input aria-label="Selling price" inputMode="decimal" value={selling} onChange={(e) => setSelling(clean(e.target.value))} placeholder={productPrice} className={INPUT} />
            </Field>
          </div>
          <Field label="Expiry" hint="Optional"><input aria-label="Expiry" type="date" value={expiry} onChange={(e) => setExpiry(e.target.value)} className={INPUT} /></Field>
          {lots.length > 0 && <BatchChoice lots={lots} value={batchId} auto={auto} unit={lotsData?.unitAbbr ?? ''} sym={sym} productPrice={productPrice} onPick={setPicked} />}
        </>
      )}
      {outcome === 'written_off' && <p className="text-[13px] text-muted-foreground">The loss stands in full.</p>}
      {needSupplier && !supplierId && (
        <Field label="Supplier">
          <select aria-label="Supplier" value={supplier} onChange={(e) => setSupplier(Number(e.target.value))} className={cn(INPUT, 'appearance-auto')}>
            <option value={0}>Pick the supplier</option>
            {sups.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
          </select>
        </Field>
      )}
      {outcome === 'replacement' && num(extra) > 0 && (
        <>
          <Seg label="The extra" value={payNow ? 'now' : 'owe'} onChange={(v) => setPayNow(v === 'now')}
            options={[['owe', `Add ${money(sym, num(extra) * num(loss.qty))} to what we owe`], ['now', 'Pay it now']]} />
          {payNow && (base === '/till'
            ? <p className="text-[13px] text-muted-foreground">Paid in cash from this till.</p>
            : <Field label="Paid from"><MoneySourcePicker method="cash" value={source} onChange={setSource} sym={sym} /></Field>)}
        </>
      )}
      {outcome === 'paid' && (
        <>
          <Field label="Amount credited">
            <input inputMode="decimal" value={amount} onChange={(e) => setAmount(e.target.value)} placeholder="0.00" className={INPUT} />
          </Field>
          <p className="text-[13px] text-muted-foreground">This takes it off what you owe them — no cash moves. If they hand the money over instead, record a refund on the supplier’s page so it lands in a locker.</p>
        </>
      )}
      <Field label="Note"><input value={note} onChange={(e) => setNote(e.target.value)} className={INPUT} /></Field>
    </FormSheet>
  )
}
