import { useQuery } from '@tanstack/react-query'
import { Search, UserPlus } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { errMsg } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import type { TillApi } from '../slots'
import { patientFromCustomer, registerPatient, searchQuery } from './api'
import { PastReceipt } from './PastReceipt'
import { PatientView } from './PatientView'
import { salePatient } from './sale'
import type { Found, Patient } from './types'

/** 💊 Patient at the till: find or register the patient, see allergies and their
 * prescriptions, add a script's medicine, put the patient on the sale — or record
 * a past receipt for them. The dispense records itself when the sale completes. */
export function PatientPanel({ till, onClose }: { till: TillApi; onClose: () => void }) {
  const [pid, setPid] = useState(salePatient(till)?.patientId ?? 0)
  const [past, setPast] = useState(false)
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[92vh] overflow-auto sm:max-w-[760px]">
        <DialogHeader>
          <DialogTitle>💊 Patient</DialogTitle>
          <DialogDescription>{pid ? 'Allergies, prescriptions and what is due.' : 'Find the patient by name, phone or NIC — or register them.'}</DialogDescription>
        </DialogHeader>
        {!pid ? <FindPatient onPick={setPid} />
          : past ? <PastReceipt patientId={pid} onBack={() => setPast(false)} onDone={onClose} />
          : <PatientView till={till} patientId={pid} onChange={() => setPid(0)} onPast={() => setPast(true)} onClose={onClose} />}
      </DialogContent>
    </Dialog>
  )
}

function FindPatient({ onPick }: { onPick: (id: number) => void }) {
  const [q, setQ] = useState('')
  const [reg, setReg] = useState(false)
  const dq = useDebounced(q.trim(), 200)
  const found = useQuery(searchQuery(dq, true)).data
  const [busy, setBusy] = useState(false)
  async function pick(f: Found) {
    if (f.patientId) return onPick(f.patientId)
    setBusy(true)
    try {
      onPick((await patientFromCustomer(f.customerId)).id)
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  if (reg) return <Register seed={q} onDone={(p) => onPick(p.id)} onCancel={() => setReg(false)} />
  return (
    <div className="grid gap-3">
      <div className="relative">
        <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-faint" />
        <input autoFocus aria-label="Find patient" placeholder="Name, phone or NIC" autoComplete="off" value={q}
          onChange={(e) => setQ(e.target.value)} className={cn(FIELD, 'pl-9')} />
      </div>
      <div className="grid max-h-[46vh] gap-1.5 overflow-auto">
        {found?.map((f) => (
          <button key={f.customerId} type="button" disabled={busy} onClick={() => void pick(f)}
            className="flex items-center gap-3 rounded-[var(--radius)] border border-border bg-card px-3 py-2.5 text-left hover:border-brand">
            <span className="grid size-9 shrink-0 place-items-center rounded-full bg-brand-soft text-sm font-bold">{f.name.slice(0, 1).toUpperCase()}</span>
            <span className="min-w-0 flex-1">
              <span className="block truncate font-semibold">{f.name}</span>
              <span className="block truncate text-xs text-muted-foreground">{[f.phone, f.nic].filter(Boolean).join(' · ') || '—'}</span>
            </span>
            {f.allergies && <span className="max-w-[40%] truncate rounded-full bg-[color-mix(in_srgb,var(--danger)_14%,transparent)] px-2 py-0.5 text-[11px] font-bold text-danger">⚠ {f.allergies}</span>}
            {!f.patientId && <span className="text-[11px] text-faint">customer</span>}
          </button>
        ))}
        {found && found.length === 0 && <p className="py-4 text-center text-sm text-muted-foreground">{dq ? `No one matches “${dq}”.` : 'No patients yet.'}</p>}
      </div>
      <Button type="button" variant="secondary" className="justify-self-start" onClick={() => setReg(true)}><UserPlus className="size-4" /> Register a new patient</Button>
    </div>
  )
}

function Register({ seed, onDone, onCancel }: { seed: string; onDone: (p: Patient) => void; onCancel: () => void }) {
  const digits = /^[\d\s+-]+$/.test(seed.trim())
  const [name, setName] = useState(digits ? '' : seed.trim())
  const [phone, setPhone] = useState(digits ? seed.trim() : '')
  const [allergies, setAllergies] = useState('')
  const [nic, setNic] = useState('')
  const [busy, setBusy] = useState(false)
  async function save() {
    setBusy(true)
    try {
      const p = await registerPatient({ name, phone, allergies, nic })
      toast.success(`${p.name} registered`)
      onDone(p)
    } catch (e) {
      toast.error(errMsg(e, 'Could not register the patient'))
    } finally {
      setBusy(false)
    }
  }
  return (
    <form autoComplete="off" className="grid gap-3 sm:grid-cols-2" onSubmit={(e) => { e.preventDefault(); void save() }}>
      <input autoFocus aria-label="Patient name" placeholder="Name" value={name} onChange={(e) => setName(e.target.value)} className={FIELD} />
      <input aria-label="Patient phone" placeholder="Phone" inputMode="tel" value={phone} onChange={(e) => setPhone(e.target.value)} className={FIELD} />
      <input aria-label="Allergies" placeholder="Allergies (e.g. Penicillin) — blank if none" value={allergies} onChange={(e) => setAllergies(e.target.value)} className={cn(FIELD, 'sm:col-span-2')} />
      <input aria-label="NIC" placeholder="NIC (optional)" value={nic} onChange={(e) => setNic(e.target.value)} className={FIELD} />
      <div className="flex items-center justify-end gap-2 sm:col-span-2">
        <Button type="button" variant="ghost" onClick={onCancel}>Back</Button>
        <Button type="submit" disabled={busy || name.trim().length < 2 || !phone.trim()}>Register</Button>
      </div>
    </form>
  )
}
