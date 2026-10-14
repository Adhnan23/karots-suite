import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { HandCoins, RotateCcw, Undo2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { CreditForm, type CreditFormOpts } from '@/features/counter/credit/CreditForm'
import { afterSlip, moneySlip } from '@/features/counter/slip'
import { configQuery as tillConfigQuery, customersQuery, errMsg } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { linkDeposit, refundHeld, startRework } from './api'
import type { Job } from './types'

/** The job's money at the till — always core's: a deposit is the customer's
 * advance (the core credit form), a cancelled job's deposit goes back the same
 * way, a karots-pos held deposit comes back from the drawer, and a faulty part
 * goes back through core Returns. */
export function JobMoney({ job, sym, onOpen }: { job: Job; sym: string; onOpen: (id: number) => void }) {
  const cfg = useQuery(tillConfigQuery).data
  const cust = useQuery(customersQuery).data?.find((c) => c.id === job.customerId)
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [form, setForm] = useState<CreditFormOpts | null>(null)
  const open = job.status !== 'collected' && job.status !== 'cancelled'
  const advance = Math.max(0, -num(job.outstanding))
  const held = num(job.legacyHeld)
  const deposits = num(job.depositTotal)
  // What a cancelled job hands back: its deposits, as far as the advance still has them.
  const giveBack = job.status === 'cancelled' ? Math.min(deposits, advance) : 0

  async function backHeld() {
    try {
      const r = await refundHeld(job.id)
      if (cfg) afterSlip(cfg, `Give ${job.customerName} ${money(sym, num(r.amount))} — deposit refunded`, moneySlip(r.receiptId), true)
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  async function rework() {
    try {
      const j = await startRework(job.id)
      toast.success(`${j.ticketNo} opened — free under ${job.ticketNo}'s warranty`)
      onOpen(j.id)
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  if (form && cust) {
    return (
      <section aria-label="Deposit" className="overflow-hidden rounded-[var(--radius)] border border-brand">
        <CreditForm key={form.mode} c={cust} opts={{
          ...form, onDone: (r) => {
            setForm(null)
            if (r.mode === 'pay' && r.paymentId) void linkDeposit(job.id, r.paymentId).catch((e) => toast.error(errMsg(e)))
            else void qc.invalidateQueries({ queryKey: ['x', 'repairs'] }) // the advance changed
          },
        }} />
        <div className="border-t border-border p-2 text-right"><Button type="button" variant="ghost" size="sm" onClick={() => setForm(null)}>Close</Button></div>
      </section>
    )
  }
  return (
    <section aria-label="Money" className="grid gap-2 rounded-[var(--radius)] border border-border p-3">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
        <span>Deposit <b className="font-display tabular-nums">{money(sym, deposits + held)}</b></span>
        {job.deposits.map((d) => <span key={d.paymentId} className="text-[12px] text-muted-foreground">{d.receiptNo} · {money(sym, num(d.amount))}</span>)}
        {held > 0 && <span className="text-[12px] text-muted-foreground">{money(sym, held)} held from before</span>}
        {cust && <span className="ml-auto text-[12px] text-muted-foreground">{advance > 0 ? `Advance on account ${money(sym, advance)}` : num(job.outstanding) > 0 ? `Owes ${money(sym, num(job.outstanding))}` : 'Nothing on account'}</span>}
      </div>
      <div className="flex flex-wrap gap-2">
        {open && !job.reworkOf && cust && (
          <Button type="button" variant="secondary" onClick={() => setForm({ mode: 'pay', amount: '', note: `Deposit — ${job.ticketNo}` })}><HandCoins className="size-4" /> Take a deposit</Button>
        )}
        {giveBack > 0 && cust && (
          <Button type="button" onClick={() => setForm({ mode: 'refund', amount: giveBack.toFixed(2) })}><Undo2 className="size-4" /> Give back the deposit {money(sym, giveBack)}</Button>
        )}
        {held > 0 && !open && <Button type="button" onClick={() => void backHeld()}><Undo2 className="size-4" /> Give back {money(sym, held)} held</Button>}
        {job.status === 'collected' && advance > 0 && cust && (
          <Button type="button" variant="secondary" onClick={() => setForm({ mode: 'refund', amount: advance.toFixed(2) })}><Undo2 className="size-4" /> Refund the advance left {money(sym, advance)}</Button>
        )}
        {job.status === 'collected' && job.inWarranty && (
          <>
            <Button type="button" variant="secondary" onClick={() => void rework()}><RotateCcw className="size-4" /> Free re-repair</Button>
            {job.saleId && <Button type="button" variant="secondary" onClick={() => void navigate({ to: '/till/returns', search: { sale: job.saleId! } })}>Return a faulty part</Button>}
          </>
        )}
      </div>
    </section>
  )
}
