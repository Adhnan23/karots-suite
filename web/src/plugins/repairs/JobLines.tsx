import { useQuery } from '@tanstack/react-query'
import { Plus, Wrench, X } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import type { TillProduct } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { errMsg, searchQuery } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { money, num, qtyText } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import { addCharge, addPart, removeCharge, removePart, suggestQuery } from './api'
import type { Job } from './types'

/** The job's parts and labour, with the estimate at today's prices. Open jobs
 * can add and remove lines; the price on the bill is the cart's at pickup. */
export function JobLines({ job, sym }: { job: Job; sym: string }) {
  const open = job.status !== 'collected' && job.status !== 'cancelled'
  const byPart = job.warrantyMode === 'parts'
  const act = (p: Promise<unknown>) => p.catch((e) => toast.error(errMsg(e)))
  const empty = job.parts.length === 0 && job.charges.length === 0
  if (empty && !open) return null
  return (
    <section aria-label="Parts and labour" className="grid gap-2 rounded-[var(--radius)] border border-border p-3">
      <div className="flex items-baseline justify-between">
        <h3 className="font-display text-[15px] font-bold">Parts & labour</h3>
        <span className="text-[13px] text-muted-foreground">{job.saleReceipt ? <>Paid on <b className="text-foreground">{job.saleReceipt}</b></> : job.reworkOf ? 'Free — warranty re-repair' : <>Estimate <b className="font-display text-base text-foreground tabular-nums">{money(sym, num(job.estimate))}</b></>}</span>
      </div>
      {empty && <p className="text-[13px] text-faint">Nothing yet — add the parts used and the labour.</p>}
      <ul className="grid gap-1">
        {job.parts.map((p) => (
          <li key={`p${p.id}`} className="flex items-center gap-2 rounded-[var(--radius-sm)] bg-muted px-2.5 py-1.5 text-sm">
            <span className="min-w-0 flex-1 truncate font-semibold">{p.name}</span>
            {byPart && p.warrantyDays > 0 && <span className="text-[11px] text-muted-foreground">{p.warrantyDays}d warranty</span>}
            {p.oldDiscount && <span className="text-[11px] text-muted-foreground">−{p.oldDiscount}</span>}
            <span className="tabular-nums text-muted-foreground">{qtyText(num(p.qty))} × {money(sym, num(p.price))}</span>
            {open && <button type="button" aria-label={`Remove ${p.name}`} onClick={() => void act(removePart(job.id, p.id))} className="rounded p-1 text-faint hover:text-danger"><X className="size-4" /></button>}
          </li>
        ))}
        {job.charges.map((c) => (
          <li key={`c${c.id}`} className="flex items-center gap-2 rounded-[var(--radius-sm)] bg-muted px-2.5 py-1.5 text-sm">
            <Wrench className="size-3.5 text-faint" />
            <span className="min-w-0 flex-1 truncate font-semibold">{c.label}</span>
            <span className="tabular-nums text-muted-foreground">{money(sym, num(c.amount))}</span>
            {open && <button type="button" aria-label={`Remove ${c.label}`} onClick={() => void act(removeCharge(job.id, c.id))} className="rounded p-1 text-faint hover:text-danger"><X className="size-4" /></button>}
          </li>
        ))}
      </ul>
      {open && (
        <div className="grid gap-2 md:grid-cols-2">
          <AddPart job={job} sym={sym} byPart={byPart} />
          <AddLabour job={job} sym={sym} />
        </div>
      )}
    </section>
  )
}

function AddPart({ job, sym, byPart }: { job: Job; sym: string; byPart: boolean }) {
  const [q, setQ] = useState('')
  const [pick, setPick] = useState<TillProduct | null>(null)
  const [qty, setQty] = useState('1')
  const [days, setDays] = useState('')
  const dq = useDebounced(q.trim(), 200)
  const found = useQuery({ ...searchQuery(dq), enabled: dq.length > 1 && !pick }).data?.items.filter((p) => !p.isService).slice(0, 6)
  async function add() {
    if (!pick) return
    try {
      await addPart(job.id, pick.id, qty || '1', Number(days || 0))
      setPick(null)
      setQ('')
      setQty('1')
      setDays('')
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  return (
    <div className="relative grid gap-1.5">
      {pick ? (
        <form className="flex gap-1.5" onSubmit={(e) => { e.preventDefault(); void add() }}>
          <button type="button" onClick={() => setPick(null)} className={cn(FIELD, 'min-w-0 flex-1 truncate text-left font-semibold')} title="Change the part">{pick.name}</button>
          <input aria-label="Part quantity" inputMode="decimal" value={qty} onChange={(e) => setQty(e.target.value.replace(/[^\d.]/g, ''))} className={cn(FIELD, 'w-16 text-center')} />
          {byPart && <input aria-label="Part warranty days" placeholder="days" inputMode="numeric" value={days} onChange={(e) => setDays(e.target.value.replace(/\D/g, ''))} className={cn(FIELD, 'w-16 text-center')} />}
          <Button type="submit" size="icon" aria-label="Add part"><Plus className="size-4" /></Button>
        </form>
      ) : (
        <input aria-label="Add a part" placeholder="+ Part — search or scan" value={q} onChange={(e) => setQ(e.target.value)} className={FIELD} />
      )}
      {found && found.length > 0 && (
        <ul className="absolute top-full z-10 mt-1 grid w-full gap-0.5 rounded-[var(--radius-sm)] border border-border bg-popover p-1 shadow-lg">
          {found.map((p) => (
            <li key={p.id}>
              <button type="button" onClick={() => setPick(p)} className="flex w-full justify-between gap-2 rounded px-2 py-1.5 text-left text-sm hover:bg-muted">
                <span className="truncate">{p.name}</span>
                <span className="shrink-0 tabular-nums text-muted-foreground">{money(sym, num(p.sellingPrice))} · {qtyText(num(p.stockQty))} left</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function AddLabour({ job, sym }: { job: Job; sym: string }) {
  const used = useQuery(suggestQuery).data?.labour ?? []
  const [label, setLabel] = useState('')
  const [amount, setAmount] = useState('')
  async function add() {
    if (amount.trim() === '') return void toast.error('Enter the labour amount')
    try {
      await addCharge(job.id, label, amount)
      setLabel('')
      setAmount('')
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  return (
    <form className="flex gap-1.5" onSubmit={(e) => { e.preventDefault(); void add() }}>
      <input aria-label="Labour" list="rp-labour" placeholder="+ Labour (e.g. Fitting)" value={label} onChange={(e) => setLabel(e.target.value)} className={cn(FIELD, 'min-w-0 flex-1')} />
      <input aria-label="Labour amount" placeholder={money(sym, 0)} inputMode="decimal" value={amount} onChange={(e) => setAmount(e.target.value.replace(/[^\d.]/g, ''))} className={cn(FIELD, 'w-28 tabular-nums')} />
      <Button type="submit" size="icon" aria-label="Add labour"><Plus className="size-4" /></Button>
      <datalist id="rp-labour">{used.map((l) => <option key={l} value={l} />)}</datalist>
    </form>
  )
}
