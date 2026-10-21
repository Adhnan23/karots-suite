import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, FilePlus2, Pencil } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { FormSheet } from '@/components/admin/FormSheet'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { errMsg } from '@/features/till/api'
import { qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { historyQuery, patientQuery, scriptsQuery, setScriptActive, updatePatient } from './api'
import { dueBadge, TONE } from './labels'
import { ProfileFields } from './PatientsTab'
import { useGo } from './nav'
import { ScriptSheet } from './ScriptSheet'
import type { Patient, Script } from './types'

const when = (iso: string) => new Date(iso).toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })

export function PatientPage({ id }: { id: number }) {
  const go = useGo()
  const p = useQuery(patientQuery(id)).data
  const scripts = useQuery(scriptsQuery(id)).data
  const history = useQuery(historyQuery(id)).data
  const [sheet, setSheet] = useState<'profile' | 'new' | Script | null>(null)
  if (!p) return <EmptyState title="Loading…" />
  return (
    <>
      <button type="button" onClick={() => go('')} className="mb-2 flex items-center gap-1 text-[13px] font-semibold text-muted-foreground hover:text-foreground"><ArrowLeft size={14} /> Patients</button>
      <PageHead title={p.name} sub={[p.phone, p.nic, p.dob && `born ${p.dob}`, p.sex].filter(Boolean).join(' · ') || 'Patient'}
        actions={<>
          <Button variant="outline" className="h-10 gap-1.5 px-4" onClick={() => setSheet('profile')}><Pencil size={16} /> Profile</Button>
          <Button className="h-10 gap-1.5 px-4 font-bold" onClick={() => setSheet('new')}><FilePlus2 size={16} /> Prescription</Button>
        </>} />
      <div className={cn('mb-4 rounded-[var(--radius)] border px-4 py-2.5 font-bold', p.allergies ? 'border-danger bg-[color-mix(in_srgb,var(--danger)_14%,transparent)] text-danger' : 'border-border text-muted-foreground')}>
        {p.allergies ? `⚠ Allergic to: ${p.allergies}` : 'No allergies noted'}
      </div>
      {p.notes && <p className="mb-4 text-sm text-muted-foreground">{p.notes}</p>}
      <h2 className="mb-2 font-display text-lg font-bold">Prescriptions</h2>
      <div className="mb-6 grid gap-3">
        {scripts?.length === 0 && <EmptyState title="No prescriptions yet" />}
        {scripts?.map((s) => <ScriptBlock key={s.id} s={s} onEdit={() => setSheet(s)} />)}
      </div>
      <h2 className="mb-2 font-display text-lg font-bold">Dispensed</h2>
      <div className={cn(SURFACE, 'divide-y divide-border')}>
        {history?.length === 0 && <EmptyState title="Nothing dispensed yet" />}
        {history?.map((d) => (
          <div key={d.id} className="grid gap-x-3 gap-y-0.5 px-4 py-2.5 text-sm md:grid-cols-[7rem_6rem_1fr_auto] md:items-baseline">
            <span className="flex gap-3 text-muted-foreground md:contents">
              <span>{when(d.at)}</span>
              <span className="font-mono text-[13px]">{d.receipt || '—'}</span>
            </span>
            <span>{d.items.map((i) => `${i.drug} × ${qtyText(i.qty)}${i.days ? ` (${i.days} d)` : ''}`).join(', ')}</span>
            <span className="flex flex-wrap items-center gap-2 text-[12px] text-faint md:justify-end">
              {d.returned && <Tag tone="muted">↩ returned</Tag>}
              {d.prescriber}
            </span>
          </div>
        ))}
      </div>
      {sheet === 'profile' && <ProfileSheet p={p} onClose={() => setSheet(null)} />}
      {sheet === 'new' && <ScriptSheet patientId={p.id} onClose={() => setSheet(null)} />}
      {sheet && typeof sheet === 'object' && <ScriptSheet patientId={p.id} script={sheet} onClose={() => setSheet(null)} />}
    </>
  )
}

function ScriptBlock({ s, onEdit }: { s: Script; onEdit: () => void }) {
  async function toggle() {
    try {
      await setScriptActive(s.id, !s.active)
      toast.success(s.active ? 'Stopped — no more reminders' : 'Resumed')
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  return (
    <div className={cn(SURFACE, !s.active && 'opacity-65')}>
      <div className="flex flex-wrap items-center gap-2 border-b border-border px-4 py-2.5">
        <span className="font-semibold">{s.prescriberName || 'Prescription'}</span>
        <span className="text-[13px] text-muted-foreground">{[s.prescriberReg, s.prescribedOn].filter(Boolean).join(' · ')}</span>
        {s.isRepeat && <Tag tone="brand">repeat</Tag>}
        {!s.active && <Tag tone="muted">stopped</Tag>}
        <span className="ml-auto flex gap-1.5">
          <Button size="sm" variant="ghost" onClick={onEdit}>Edit</Button>
          <Button size="sm" variant="ghost" onClick={() => void toggle()}>{s.active ? 'Stop' : 'Resume'}</Button>
        </span>
      </div>
      {s.items.map((it) => {
        const b = dueBadge(it.status, it.nextDue)
        return (
          <div key={it.id} className="flex flex-wrap items-center gap-x-3 gap-y-1 px-4 py-2 text-sm not-last:border-b not-last:border-border">
            <span className="min-w-0 flex-1">
              <span className="block font-semibold">{it.drug}{it.groupId && <small className="ml-1.5 font-normal text-faint">any brand</small>}</span>
              <span className="block text-[12px] text-muted-foreground">
                {[it.dosage, Number(it.perDay) ? `${qtyText(it.perDay)} a day` : '', Number(it.quantity) ? `${qtyText(it.quantity)} prescribed` : '', `${qtyText(it.dispensed)} given`].filter(Boolean).join(' · ')}
              </span>
            </span>
            {b && <span className={cn('rounded-pill px-2 py-0.5 text-[11px] font-bold whitespace-nowrap', TONE[b.tone])}>{b.text}</span>}
          </div>
        )
      })}
    </div>
  )
}

function ProfileSheet({ p, onClose }: { p: Patient; onClose: () => void }) {
  const [f, setF] = useState({ allergies: p.allergies, nic: p.nic, dob: p.dob, sex: p.sex, notes: p.notes })
  const [busy, setBusy] = useState(false)
  const set = (k: keyof typeof f) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value })
  async function save() {
    setBusy(true)
    try {
      await updatePatient(p.id, f)
      toast.success('Profile saved')
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={p.name} submit="Save profile" busy={busy} onSubmit={() => void save()}>
      <small className="text-faint">Name and phone are the customer’s — change them in People → Customers.</small>
      <ProfileFields f={f} set={set} />
    </FormSheet>
  )
}
