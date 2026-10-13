import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { LossRow, RecoveryRequest } from '@/api/types.gen'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { suppliersQuery } from '@/features/counter/queries'
import { Seg } from '@/features/till/bits'
import { errMsg } from '@/features/till/api'
import { money, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'

/** What a supplier made good on one loss: replaced it (back in stock), credited
 * us any amount (full or part), or nothing. Losses & Recovery and the batch
 * write-offs open it. */
export function RecoverSheet({ loss, sym, onClose }: { loss: LossRow; sym: string; onClose: () => void }) {
  const qc = useQueryClient()
  const sups = useQuery(suppliersQuery('')).data ?? []
  const [outcome, setOutcome] = useState<'replacement' | 'paid' | 'written_off'>('replacement')
  const [supplier, setSupplier] = useState(0)
  const [amount, setAmount] = useState('')
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  async function save() {
    setBusy(true)
    try {
      const body: RecoveryRequest = { sourceType: loss.kind, sourceId: loss.sourceId, outcome, supplierId: supplier, amount, note }
      await api('/admin/recovery', { json: body })
      toast.success('Recovery recorded')
      void qc.invalidateQueries({ queryKey: ['admin'] })
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Recover: ${loss.product}`} submit="Record" busy={busy} onSubmit={save}>
      <p className="text-sm text-muted-foreground">Lost {qtyText(loss.qty)} worth {money(sym, loss.loss)} at cost.</p>
      <Seg label="What the supplier did" value={outcome} onChange={setOutcome}
        options={[['replacement', 'Replaced it'], ['paid', 'Credited us'], ['written_off', 'Nothing']]} />
      {outcome === 'replacement' && <p className="text-[13px] text-muted-foreground">The replacement goes back on the shelf ({qtyText(loss.qty)} back in stock).</p>}
      {outcome === 'written_off' && <p className="text-[13px] text-muted-foreground">The loss stands in full.</p>}
      {outcome === 'paid' && (
        <>
          <Field label="Supplier">
            <select value={supplier} onChange={(e) => setSupplier(Number(e.target.value))} className={cn(INPUT, 'appearance-auto')}>
              <option value={0}>Pick the supplier</option>
              {sups.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
            </select>
          </Field>
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
