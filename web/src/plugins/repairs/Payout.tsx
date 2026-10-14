import { useQuery } from '@tanstack/react-query'
import { Banknote } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { INPUT } from '@/components/admin/fields'
import { Button } from '@/components/ui/button'
import { MoneySourcePicker } from '@/features/purchasing/MoneySource'
import { configQuery as tillConfigQuery, errMsg } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { payRepairer } from './api'
import type { Job } from './types'

/** Admin: paying the outside repairer — a core expense (Repairs) out of a cash
 * place, linked to the job. Paid / Not paid yet answers "you never paid me". */
export function Payout({ job }: { job: Job }) {
  const sym = useQuery(tillConfigQuery).data?.symbol ?? 'Rs.'
  const [amount, setAmount] = useState('')
  const [source, setSource] = useState('')
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const paid = num(job.repairerPaid)
  if (!job.repairedBy && paid === 0) return null
  async function pay() {
    if (!(num(amount) > 0)) return void toast.error('Enter the amount paid')
    if (!source) return void toast.error('Pick where the money comes from')
    setBusy(true)
    try {
      await payRepairer(job.id, amount, source, note)
      toast.success(`Paid ${job.repairedBy || 'the repairer'} ${money(sym, num(amount))} — in Expenses`)
      setAmount('')
      setNote('')
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <section aria-label="Repairer" className="grid grid-cols-[minmax(0,1fr)] gap-2 rounded-[var(--radius)] border border-border p-3">
      <div className="flex flex-wrap items-center gap-2">
        <h3 className="font-display text-[15px] font-bold">{job.repairedBy || 'Repairer'}</h3>
        <span className={cn('rounded-full px-2.5 py-0.5 text-[11px] font-bold', paid > 0 ? 'bg-[color-mix(in_srgb,var(--success)_16%,transparent)] text-success' : 'bg-[color-mix(in_srgb,var(--warning)_16%,transparent)] text-warning')}>
          {paid > 0 ? `Paid ${money(sym, paid)}` : 'Not paid yet'}
        </span>
      </div>
      {job.payouts.map((p) => <p key={p.expenseId} className="text-[12px] text-muted-foreground">{p.date} · {money(sym, num(p.amount))} · {p.note}</p>)}
      {job.status !== 'cancelled' && (
        <form className="grid grid-cols-[minmax(0,1fr)] gap-2 sm:grid-cols-[130px_1fr_1fr_auto]" onSubmit={(e) => { e.preventDefault(); void pay() }}>
          <input aria-label="Amount paid" placeholder={money(sym, 0)} inputMode="decimal" value={amount} onChange={(e) => setAmount(e.target.value.replace(/[^\d.]/g, ''))} className={cn(INPUT, 'tabular-nums')} />
          <MoneySourcePicker method="cash" value={source} onChange={setSource} sym={sym} label="Paid from" required />
          <input aria-label="Note" placeholder="Note (optional)" value={note} onChange={(e) => setNote(e.target.value)} className={INPUT} />
          <Button type="submit" disabled={busy}><Banknote className="size-4" /> Pay</Button>
        </form>
      )}
    </section>
  )
}
