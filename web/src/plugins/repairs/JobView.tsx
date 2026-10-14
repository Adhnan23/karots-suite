import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, MessageCircle, Pencil, Phone, Printer, ReceiptText, Tag } from 'lucide-react'
import type { ReactNode } from 'react'
import { toast } from 'sonner'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { Button } from '@/components/ui/button'
import { Seg } from '@/features/till/bits'
import { configQuery as tillConfigQuery, errMsg } from '@/features/till/api'
import { printSlip } from '@/features/till/print'
import { cn } from '@/lib/utils'
import type { TillApi } from '../slots'
import { cancelJob, jobQuery, printTicketLabel, setStatus, setUrgent, slipPath } from './api'
import { JobLines } from './JobLines'
import { JobMoney } from './JobMoney'
import { putOnBill } from './till'
import type { Job } from './types'
import { device, readyMessage, STATUS, statusTone, waNumber } from './words'

const when = (iso: string) => (iso ? new Date(iso).toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' }) : '')

/** One job: who and what, its status, parts & labour, and what can happen next.
 * With a till (the 🔧 panel) it also takes deposits and Puts it on the bill;
 * without one (admin) extra shows the admin's part (paying the repairer). */
export function JobView({ id, till, onBack, onOpen, onEdit, onDone, extra }: {
  id: number; till?: TillApi; onBack: () => void; onOpen: (id: number) => void; onEdit: () => void; onDone?: () => void; extra?: (j: Job) => ReactNode
}) {
  const job = useQuery(jobQuery(id)).data
  const cfg = useQuery(tillConfigQuery).data
  if (!job || !cfg) return <p className="py-8 text-center text-sm text-muted-foreground">Loading…</p>
  const sym = cfg.symbol
  const open = job.status !== 'collected' && job.status !== 'cancelled'
  const act = (p: Promise<unknown>) => p.catch((e) => toast.error(errMsg(e)))
  async function bill() {
    if (!till || !job) return
    const err = await putOnBill(till, job).catch((e) => errMsg(e))
    if (err) return void toast.error(err)
    toast.success(`${job.ticketNo} is on the bill — take payment as usual`)
    onDone?.()
  }
  async function label() {
    try {
      await printTicketLabel(job!)
      toast.success('Ticket label sent to the label printer')
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  return (
    <div className="grid grid-cols-[minmax(0,1fr)] gap-3">
      <div className="flex flex-wrap items-start gap-3">
        <button type="button" aria-label="Back to the list" onClick={onBack} className="mt-1 rounded-full p-1.5 text-muted-foreground hover:bg-muted"><ArrowLeft className="size-4" /></button>
        <div className="grid min-w-[68px] place-items-center rounded-[var(--radius)] bg-brand-soft px-2.5 py-2 text-center">
          <span className="font-display text-lg leading-none font-bold tabular-nums">{job.ticketNo}</span>
          <span className="mt-1 text-[10px] tracking-wide text-muted-foreground">{job.ticketCode}</span>
        </div>
        <div className="min-w-0 flex-1">
          <h3 className="truncate font-display text-xl font-bold">{device(job)}</h3>
          <p className="text-[13px] text-muted-foreground">
            In {when(job.createdAt)}{job.promisedDate && <> · ready by <b className={cn(job.dueUrgent && open && 'text-danger')}>{when(job.promisedDate)}</b></>}
            {job.due && open && <span className={cn('ml-1.5 rounded-full px-2 py-0.5 text-[11px] font-bold', job.dueUrgent ? 'bg-[color-mix(in_srgb,var(--danger)_14%,transparent)] text-danger' : 'bg-muted')}>{job.due}</span>}
            {job.collectedAt && <> · collected {when(job.collectedAt)}</>}
          </p>
        </div>
        <span className={cn('rounded-full px-3 py-1 text-[12px] font-bold', statusTone[job.status])}>{STATUS[job.status]}</span>
      </div>

      <div className="grid gap-2 rounded-[var(--radius)] bg-muted p-3 text-sm sm:grid-cols-2">
        <div className="min-w-0">
          <b className="block truncate">{job.customerName || '—'}</b>
          {job.customerPhone && (
            <span className="mt-1 flex flex-wrap gap-1.5">
              <a href={`tel:${job.customerPhone}`} className="inline-flex h-7 items-center gap-1 rounded-full border border-border bg-card px-2.5 text-[12px] font-semibold hover:border-brand"><Phone size={12} />{job.customerPhone}</a>
              {job.status === 'ready' && (
                <a href={`https://wa.me/${waNumber(job.customerPhone)}?text=${encodeURIComponent(readyMessage(job, cfg.shop.name))}`} target="_blank" rel="noreferrer"
                  className="inline-flex h-7 items-center gap-1 rounded-full border border-border bg-card px-2.5 text-[12px] font-semibold hover:border-brand"><MessageCircle size={12} />Tell them it's ready</a>
              )}
            </span>
          )}
        </div>
        <div className="grid gap-0.5 text-[13px]">
          {job.fault && <span><span className="text-muted-foreground">Fault · </span>{job.fault}</span>}
          {job.notes && <span><span className="text-muted-foreground">Notes · </span>{job.notes}</span>}
          <span className="text-muted-foreground">{job.repairedBy ? `Repaired by ${job.repairedBy}` : 'Repaired here'}
            {job.warrantyMode === 'days' && (job.warrantyUntil ? ` · warranty until ${when(job.warrantyUntil)}` : job.warrantyDays > 0 ? ` · ${job.warrantyDays} days warranty` : ' · no warranty')}
            {job.reworkOf && ' · warranty re-repair (free)'}</span>
        </div>
      </div>

      {open && (
        <div className="flex flex-wrap items-center gap-2">
          <Seg label="Status" value={job.status as 'received' | 'in_progress' | 'ready'} onChange={(s) => void act(setStatus(job.id, s))}
            options={[['received', 'Received'], ['in_progress', 'In progress'], ['ready', 'Ready']]} />
          <label className="ml-auto flex items-center gap-2 text-sm font-semibold">
            <input type="checkbox" checked={job.urgent} onChange={(e) => void act(setUrgent(job.id, e.target.checked))} className="size-4 accent-[var(--danger)]" /> Urgent
          </label>
        </div>
      )}

      <JobLines job={job} sym={sym} />
      {till && <JobMoney job={job} sym={sym} onOpen={onOpen} />}
      {extra?.(job)}

      <div className="flex flex-wrap items-center gap-2 border-t border-border pt-3">
        <Button type="button" variant="ghost" size="sm" onClick={() => void label()}><Tag className="size-4" /> Label</Button>
        <Button type="button" variant="ghost" size="sm" onClick={() => void printSlip(slipPath(job.id))}><Printer className="size-4" /> Slip</Button>
        {open && <Button type="button" variant="ghost" size="sm" onClick={onEdit}><Pencil className="size-4" /> Edit</Button>}
        {open && (
          <ConfirmButton plain label="Cancel job" title={`Cancel ${job.ticketNo}?`} confirm="Cancel the job"
            body={num0(job.depositTotal) + num0(job.legacyHeld) > 0 ? 'The deposit is offered back next.' : 'The device goes back as it is.'}
            onConfirm={() => act(cancelJob(job.id))} className="text-danger" />
        )}
        {till && open && (
          <Button type="button" className="ml-auto" onClick={() => void bill()}><ReceiptText className="size-4" /> Put on the bill</Button>
        )}
      </div>
    </div>
  )
}

const num0 = (s: string) => Number(s) || 0
