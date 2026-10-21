import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { Plus, Search, Zap } from 'lucide-react'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Seg } from '@/features/till/bits'
import { configQuery as tillConfigQuery } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { money } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import type { TillApi } from '../slots'
import { jobsQuery, labourQuery, suggestQuery, type Show } from './api'
import { JobForm } from './JobForm'
import { JobView } from './JobView'
import { isTicket } from './till'
import type { Row } from './types'
import { device, STATUS, statusTone } from './words'

type View = { kind: 'home' } | { kind: 'new' } | { kind: 'quick' } | { kind: 'job'; id: number } | { kind: 'edit'; id: number }

/** 🔧 Repairs at the till: find a job (or scan its ticket), take one in, work on
 * it, take a deposit and Put it on the bill — paid through the normal Pay. */
export function RepairsPanel({ till, onClose, text }: { till: TillApi; onClose: () => void; text?: string }) {
  const [view, setView] = useState<View>({ kind: 'home' })
  const [q, setQ] = useState(text && isTicket(text) ? text.trim() : '')
  const scanned = useQuery({ ...jobsQuery('all', q), enabled: !!text && isTicket(text) }).data
  // A scanned ticket opens its job straight away.
  useEffect(() => {
    const hit = scanned?.find((r) => [r.ticketCode, r.ticketNo].some((x) => x.toUpperCase() === q.toUpperCase()))
    if (hit) setView({ kind: 'job', id: hit.id })
  }, [scanned, q])
  const title = view.kind === 'new' ? 'New repair job' : view.kind === 'quick' ? '⚡ Quick fix' : view.kind === 'edit' ? 'Edit the job' : '🔧 Repairs'
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[92vh] overflow-auto sm:max-w-[820px]">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>
            {view.kind === 'home' ? 'Scan or type a ticket, or find the customer. Pickup is paid on the normal Pay screen.'
              : view.kind === 'quick' ? 'Fixed on the spot — it goes on the bill as labour; scan any parts as usual.'
              : view.kind === 'new' ? 'What was left with us, and how we reach the customer.' : ' '}
          </DialogDescription>
        </DialogHeader>
        {view.kind === 'home' && <Home q={q} setQ={setQ} onOpen={(id) => setView({ kind: 'job', id })} onNew={() => setView({ kind: 'new' })} onQuick={() => setView({ kind: 'quick' })} />}
        {view.kind === 'new' && <JobForm till={till} onSaved={(id) => setView({ kind: 'job', id })} onCancel={() => setView({ kind: 'home' })} />}
        {view.kind === 'edit' && <JobForm till={till} jobId={view.id} onSaved={(id) => setView({ kind: 'job', id })} onCancel={() => setView({ kind: 'job', id: view.id })} />}
        {view.kind === 'quick' && <QuickFix till={till} onDone={onClose} onBack={() => setView({ kind: 'home' })} />}
        {view.kind === 'job' && <JobView id={view.id} till={till} onBack={() => setView({ kind: 'home' })} onOpen={(id) => setView({ kind: 'job', id })}
          onEdit={() => setView({ kind: 'edit', id: view.id })} onDone={onClose} />}
      </DialogContent>
    </Dialog>
  )
}

function Home({ q, setQ, onOpen, onNew, onQuick }: { q: string; setQ: (v: string) => void; onOpen: (id: number) => void; onNew: () => void; onQuick: () => void }) {
  const [show, setShow] = useState<Show>('open')
  const dq = useDebounced(q.trim(), 200)
  const rows = useQuery(jobsQuery(dq ? 'all' : show, dq)).data
  return (
    <div className="grid gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-[220px] flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-faint" />
          <input autoFocus aria-label="Find a repair" placeholder="Ticket, name, phone or device" autoComplete="off" value={q}
            onChange={(e) => setQ(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && rows?.length === 1 && onOpen(rows[0].id)}
            className={cn(FIELD, 'w-full pl-9')} />
        </div>
        <Button type="button" onClick={onNew}><Plus className="size-4" /> New job</Button>
        <Button type="button" variant="secondary" onClick={onQuick}><Zap className="size-4" /> Quick fix</Button>
      </div>
      {!dq && <Seg label="Show" value={show} onChange={setShow} options={[['open', 'Open'], ['ready', 'Ready'], ['collected', 'Collected'], ['cancelled', 'Cancelled']]} />}
      <ul aria-label="Repair jobs" className="grid max-h-[52vh] gap-1.5 overflow-auto">
        {rows?.map((r) => <JobRow key={r.id} r={r} onOpen={() => onOpen(r.id)} />)}
        {rows && rows.length === 0 && (
          <li className="py-8 text-center text-sm text-muted-foreground">{dq ? `No repair matches “${dq}”.` : show === 'open' ? 'Nothing in for repair right now.' : 'None here.'}</li>
        )}
      </ul>
    </div>
  )
}

export function JobRow({ r, onOpen }: { r: Row; onOpen: () => void }) {
  return (
    <li>
      <button type="button" onClick={onOpen}
        className={cn('flex w-full items-center gap-3 rounded-[var(--radius)] border bg-card px-3 py-2.5 text-left hover:border-brand',
          r.dueUrgent ? 'border-[color-mix(in_srgb,var(--danger)_45%,var(--border))]' : 'border-border')}>
        <span className="grid min-w-[64px] place-items-center rounded-[var(--radius-sm)] bg-brand-soft px-2 py-1.5 font-display text-[13px] font-bold tabular-nums">{r.ticketNo}</span>
        <span className="min-w-0 flex-1">
          <span className="block truncate font-semibold">{device(r)}</span>
          <span className="block truncate text-xs text-muted-foreground">{[r.customerName, r.customerPhone].filter(Boolean).join(' · ') || '—'}</span>
        </span>
        {r.due && <span className={cn('rounded-pill px-2 py-0.5 text-[11px] font-bold', r.dueUrgent ? 'bg-[color-mix(in_srgb,var(--danger)_14%,transparent)] text-danger' : 'bg-muted text-muted-foreground')}>{r.due}</span>}
        {!r.due && r.dueUrgent && <span className="rounded-pill bg-[color-mix(in_srgb,var(--danger)_14%,transparent)] px-2 py-0.5 text-[11px] font-bold text-danger">Urgent</span>}
        <span className={cn('rounded-pill px-2.5 py-0.5 text-[11px] font-bold', statusTone[r.status])}>{STATUS[r.status]}</span>
      </button>
    </li>
  )
}

const PRESETS = ['SIM reseat', 'Software / settings', 'Battery swap', 'Screen guard', 'Cleaning', 'Charging port clean']

/** Quick fix: a labour line straight onto the bill — no job. */
function QuickFix({ till, onDone, onBack }: { till: TillApi; onDone: () => void; onBack: () => void }) {
  const sym = useSuspenseQuery(tillConfigQuery).data.symbol
  const labour = useQuery(labourQuery).data
  const used = useQuery(suggestQuery).data?.labour ?? []
  const [what, setWhat] = useState('')
  const [price, setPrice] = useState('')
  const picks = [...new Set([...PRESETS, ...used])].slice(0, 12)
  function add() {
    if (!labour) return
    if (!what.trim()) return void toast.error('Say what you fixed')
    if (price.trim() === '') return void toast.error('Enter the price')
    till.addServiceLine({ productId: labour.productId, name: `Repair — ${what.trim()}`, price: Number(price) })
    toast.success(`Repair — ${what.trim()} · ${money(sym, Number(price))}`)
    onDone()
  }
  return (
    <form autoComplete="off" className="grid gap-3" onSubmit={(e) => { e.preventDefault(); add() }}>
      <div className="flex flex-wrap gap-1.5">
        {picks.map((p) => (
          <button key={p} type="button" aria-pressed={what === p} onClick={() => setWhat(p)}
            className="rounded-pill border border-border px-3 py-1 text-[13px] font-semibold hover:border-brand aria-pressed:border-brand aria-pressed:bg-brand aria-pressed:text-brand-foreground">{p}</button>
        ))}
      </div>
      <div className="grid gap-2 sm:grid-cols-[1fr_180px]">
        <input autoFocus aria-label="What was fixed" placeholder="What was fixed" value={what} onChange={(e) => setWhat(e.target.value)} className={FIELD} />
        <input aria-label="Price" placeholder={money(sym, 0)} inputMode="decimal" value={price} onChange={(e) => setPrice(e.target.value.replace(/[^\d.]/g, ''))}
          className={cn(FIELD, 'font-display text-lg font-bold tabular-nums')} />
      </div>
      <div className="flex justify-end gap-2">
        <Button type="button" variant="ghost" onClick={onBack}>Back</Button>
        <Button type="submit" disabled={!labour}>Add to the bill</Button>
      </div>
    </form>
  )
}
