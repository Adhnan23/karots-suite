import { useQuery } from '@tanstack/react-query'
import { UserPlus } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { DataList } from '@/components/admin/DataList'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { EmptyState } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { errMsg } from '@/features/till/api'
import { useDebounced } from '@/lib/useDebounced'
import { refillsQuery, registerPatient, searchQuery } from './api'
import { useGo } from './nav'
import type { Found } from './types'

export function PatientsTab() {
  const go = useGo()
  const [q, setQ] = useState('')
  const dq = useDebounced(q.trim(), 200)
  const rows = useQuery(searchQuery(dq)).data
  const due = useQuery(refillsQuery).data ?? []
  const [reg, setReg] = useState(false)
  const dueOf = (id: number) => due.filter((r) => r.patientId === id).length
  return (
    <>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <input aria-label="Search patients" placeholder="Name, phone or NIC" value={q} onChange={(e) => setQ(e.target.value)} className={`${INPUT} h-10 w-full md:w-72`} />
        <Button className="h-10 gap-1.5 px-4 font-bold md:ml-auto" onClick={() => setReg(true)}><UserPlus size={16} /> New patient</Button>
      </div>
      <DataList<Found> label="Patients" rows={rows ?? []} rowKey={(r) => r.patientId} onRow={(r) => go(`patients/${r.patientId}`)}
        empty={<EmptyState title={!rows ? 'Loading…' : dq ? 'Nobody matches' : 'No patients yet'}>{rows && !dq && 'Register one here or from the till’s 💊 Patient.'}</EmptyState>}
        columns={[
          { key: 'name', header: 'Patient', cell: (r) => r.name },
          { key: 'phone', header: 'Phone', cell: (r) => r.phone || '—' },
          { key: 'nic', header: 'NIC', cell: (r) => r.nic || '—', wide: true },
          { key: 'allergies', header: 'Allergies', cell: (r) => (r.allergies ? <Tag tone="danger">{`⚠ ${r.allergies}`}</Tag> : '—') },
          { key: 'due', header: 'Refills due', cell: (r) => (dueOf(r.patientId) ? <Tag tone="warn">{`${dueOf(r.patientId)} due`}</Tag> : '—') },
        ]} />
      {reg && <RegisterSheet onClose={() => setReg(false)} onDone={(id) => go(`patients/${id}`)} />}
    </>
  )
}

function RegisterSheet({ onClose, onDone }: { onClose: () => void; onDone: (id: number) => void }) {
  const [f, setF] = useState({ name: '', phone: '', allergies: '', nic: '', dob: '', sex: '', notes: '' })
  const [busy, setBusy] = useState(false)
  const set = (k: keyof typeof f) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value })
  async function save() {
    setBusy(true)
    try {
      const p = await registerPatient(f)
      toast.success(`${p.name} registered`)
      onDone(p.id)
    } catch (e) {
      toast.error(errMsg(e, 'Could not register the patient'))
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title="New patient" submit="Register" busy={busy} onSubmit={() => void save()}>
      <div className="grid gap-3 md:grid-cols-2">
        <Field label="Name"><input aria-label="Patient name" value={f.name} onChange={set('name')} className={INPUT} /></Field>
        <Field label="Phone" hint="An existing customer with this phone becomes the patient."><input aria-label="Patient phone" inputMode="tel" value={f.phone} onChange={set('phone')} className={INPUT} /></Field>
      </div>
      <ProfileFields f={f} set={set} />
    </FormSheet>
  )
}

type Prof = { allergies: string; nic: string; dob: string; sex: string; notes: string }
export function ProfileFields({ f, set }: { f: Prof; set: (k: keyof Prof) => (e: { target: { value: string } }) => void }) {
  return (
    <>
      <Field label="Allergies" hint="Shown in red at the till."><input aria-label="Allergies" value={f.allergies} onChange={set('allergies')} className={INPUT} /></Field>
      <div className="grid gap-3 md:grid-cols-3">
        <Field label="NIC"><input aria-label="NIC" value={f.nic} onChange={set('nic')} className={INPUT} /></Field>
        <Field label="Date of birth"><input aria-label="Date of birth" type="date" value={f.dob} onChange={set('dob')} className={INPUT} /></Field>
        <Field label="Sex">
          <select aria-label="Sex" value={f.sex} onChange={set('sex')} className={`${INPUT} appearance-auto`}>
            <option value="">—</option><option value="F">Female</option><option value="M">Male</option>
          </select>
        </Field>
      </div>
      <Field label="Notes"><input aria-label="Patient notes" value={f.notes} onChange={set('notes')} className={INPUT} /></Field>
    </>
  )
}
