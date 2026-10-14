import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Search, UserPlus, X } from 'lucide-react'
import { useMemo, useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { CustomerCreated, TillCustomer } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { customersQuery, errMsg } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { cn } from '@/lib/utils'
import type { TillApi } from '../slots'
import { configQuery, createJob, jobQuery, suggestQuery, updateJob } from './api'
import type { Job, JobInput } from './types'

const LABEL = 'grid gap-1 text-[12px] font-semibold text-muted-foreground'

/** Take a job in (or edit it): who it belongs to — a customer we can call — and
 * what was left with us. */
export function JobForm({ till, jobId = 0, onSaved, onCancel }: { till?: TillApi; jobId?: number; onSaved: (id: number) => void; onCancel: () => void }) {
  const job = useQuery(jobQuery(jobId)).data
  if (jobId && !job) return <p className="py-8 text-center text-sm text-muted-foreground">Loading…</p>
  return <Form key={job?.id ?? 0} job={job} seedCustomer={till?.customerId ?? null} onSaved={onSaved} onCancel={onCancel} />
}

function Form({ job, seedCustomer, onSaved, onCancel }: { job?: Job; seedCustomer: number | null; onSaved: (id: number) => void; onCancel: () => void }) {
  const cfg = useQuery(configQuery).data
  const sug = useQuery(suggestQuery).data
  const [customerId, setCustomerId] = useState<number | null>(job?.customerId ?? seedCustomer)
  const [f, setF] = useState({
    deviceModel: job?.deviceModel ?? '', repairType: job?.repairType ?? '', fault: job?.fault ?? '', repairedBy: job?.repairedBy ?? '',
    notes: job?.notes ?? '', promisedDate: job?.promisedDate ?? '', urgent: job?.urgent ?? false,
    warrantyDays: job ? String(job.warrantyDays) : '',
  })
  const [busy, setBusy] = useState(false)
  const set = (k: keyof typeof f) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value })
  const byPart = cfg?.warrantyMode === 'parts'
  async function save() {
    if (!customerId) return void toast.error('Pick the customer — we must be able to call them')
    if (!f.deviceModel.trim() && !f.repairType.trim()) return void toast.error('Say what device it is or what the repair is')
    const body: JobInput = { customerId, deviceModel: f.deviceModel, repairType: f.repairType, fault: f.fault, repairedBy: f.repairedBy,
      notes: f.notes, promisedDate: f.promisedDate, urgent: f.urgent }
    if (!byPart && f.warrantyDays.trim() !== '') body.warrantyDays = Number(f.warrantyDays)
    if (byPart) body.warrantyDays = 0
    setBusy(true)
    try {
      const j = job ? await updateJob(job.id, body) : await createJob(body)
      if (!job) toast.success(`${j.ticketNo} taken in`)
      onSaved(j.id)
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <form autoComplete="off" className="grid gap-3 sm:grid-cols-2" onSubmit={(e) => { e.preventDefault(); void save() }}>
      <div className="sm:col-span-2"><CustomerPick id={customerId} onPick={setCustomerId} /></div>
      <label className={LABEL}>Device
        <input autoFocus={!!customerId} list="rp-models" placeholder="e.g. Galaxy A12" value={f.deviceModel} onChange={set('deviceModel')} className={FIELD} />
      </label>
      <label className={LABEL}>Repair
        <input list="rp-types" placeholder="e.g. Screen replacement" value={f.repairType} onChange={set('repairType')} className={FIELD} />
      </label>
      <label className={cn(LABEL, 'sm:col-span-2')}>Fault the customer reports
        <textarea rows={2} placeholder="What's wrong, and anything left with it (charger, case, SIM)" value={f.fault} onChange={set('fault')}
          className={cn(FIELD, 'h-auto py-2')} />
      </label>
      <label className={LABEL}>Ready by
        <input type="date" value={f.promisedDate} onChange={set('promisedDate')} className={FIELD} />
      </label>
      <label className={LABEL}>Repaired by <span className="font-normal text-faint">blank = us</span>
        <input list="rp-repairers" placeholder="Us" value={f.repairedBy} onChange={set('repairedBy')} className={FIELD} />
      </label>
      {!byPart && (
        <label className={LABEL}>Warranty (days)
          <input inputMode="numeric" placeholder={String(cfg?.defaultWarrantyDays ?? 0)} value={f.warrantyDays}
            onChange={(e) => setF({ ...f, warrantyDays: e.target.value.replace(/\D/g, '') })} className={FIELD} />
        </label>
      )}
      <label className="flex items-center gap-2 self-end pb-2 text-sm font-semibold">
        <input type="checkbox" checked={f.urgent} onChange={(e) => setF({ ...f, urgent: e.target.checked })} className="size-4 accent-[var(--danger)]" /> Urgent
      </label>
      <label className={cn(LABEL, 'sm:col-span-2')}>Notes <span className="font-normal text-faint">for the shop</span>
        <input placeholder="Passcode given, condition, …" value={f.notes} onChange={set('notes')} className={FIELD} />
      </label>
      <datalist id="rp-models">{sug?.models.map((m) => <option key={m} value={m} />)}</datalist>
      <datalist id="rp-types">{sug?.types.map((m) => <option key={m} value={m} />)}</datalist>
      <datalist id="rp-repairers">{sug?.repairers.map((m) => <option key={m} value={m} />)}</datalist>
      <div className="flex justify-end gap-2 sm:col-span-2">
        <Button type="button" variant="ghost" onClick={onCancel}>Back</Button>
        <Button type="submit" disabled={busy}>{job ? 'Save' : 'Take it in'}</Button>
      </div>
    </form>
  )
}

/** Find a customer by name or phone, or add one (name + phone, like the till's). */
function CustomerPick({ id, onPick }: { id: number | null; onPick: (id: number | null) => void }) {
  const qc = useQueryClient()
  const all = useQuery(customersQuery).data ?? []
  const [q, setQ] = useState('')
  const [adding, setAdding] = useState(false)
  const [phone, setPhone] = useState('')
  const cur = all.find((c) => c.id === id)
  const list = useMemo(() => {
    const s = q.trim().toLowerCase()
    return s ? all.filter((c) => c.name.toLowerCase().includes(s) || c.phone.includes(s)).slice(0, 6) : []
  }, [all, q])
  async function add() {
    const digits = /^[\d\s+-]+$/.test(q.trim())
    const name = digits ? '' : q.trim()
    const ph = digits ? q.trim() : phone.trim()
    if (!name || !ph) return void toast.error('Enter the name and phone')
    try {
      const r = await api<CustomerCreated>('/till/customers', { json: { name, phone: ph, creditLimit: '0' } })
      await qc.invalidateQueries({ queryKey: customersQuery.queryKey })
      toast.success(r.existed ? `${r.customer.name} is already a customer — using them` : `${r.customer.name} added`)
      onPick(r.customer.id)
      setAdding(false)
      setQ('')
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  if (cur) return <Picked c={cur} onClear={() => onPick(null)} />
  return (
    <div className="grid gap-1.5">
      <span className="text-[12px] font-semibold text-muted-foreground">Customer</span>
      <div className="flex gap-2">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-faint" />
          <input autoFocus aria-label="Find customer" placeholder="Name or phone" value={q} onChange={(e) => setQ(e.target.value)} className={cn(FIELD, 'w-full pl-9')} />
        </div>
        {adding && <input aria-label="Phone" placeholder="Phone" inputMode="tel" value={phone} onChange={(e) => setPhone(e.target.value)} className={cn(FIELD, 'w-40')} />}
        <Button type="button" variant="secondary" onClick={() => (adding ? void add() : setAdding(true))}><UserPlus className="size-4" /> {adding ? 'Add' : 'New'}</Button>
      </div>
      {list.length > 0 && (
        <ul aria-label="Customers" className="grid gap-1">
          {list.map((c) => (
            <li key={c.id}>
              <button type="button" onClick={() => onPick(c.id)} className="flex w-full items-center justify-between rounded-[var(--radius-sm)] border border-border px-3 py-2 text-left text-sm hover:border-brand">
                <b>{c.name}</b><span className="text-muted-foreground tabular-nums">{c.phone || 'no phone'}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function Picked({ c, onClear }: { c: TillCustomer; onClear: () => void }) {
  return (
    <div className="flex items-center gap-3 rounded-[var(--radius)] border border-border bg-muted px-3 py-2">
      <span className="grid size-9 place-items-center rounded-full bg-brand-soft text-sm font-bold">{c.name.slice(0, 1).toUpperCase()}</span>
      <span className="min-w-0 flex-1">
        <b className="block truncate">{c.name}</b>
        <small className="text-muted-foreground tabular-nums">{c.phone || 'no phone'}</small>
      </span>
      <button type="button" aria-label="Change customer" onClick={onClear} className="rounded-full p-1.5 text-muted-foreground hover:bg-card"><X className="size-4" /></button>
    </div>
  )
}
