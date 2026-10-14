import { useQuery } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { DataList } from '@/components/admin/DataList'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { EmptyState } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { errMsg } from '@/features/till/api'
import { qtyText } from '@/lib/money'
import { createDose, deleteDose, dosesQuery, updateDose } from './api'
import { dosesPerDay } from './dosage'
import type { Dose } from './types'

/** Pharmacy → Doses: how a dose is written, what it means on the dose label, doses a day.
 * Starts with the usual ones; doses used elsewhere join by themselves (to be named). */
export function DosesTab() {
  const rows = useQuery(dosesQuery).data
  const [edit, setEdit] = useState<Dose | 'new' | null>(null)
  return (
    <>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <p className="text-[13px] text-muted-foreground">Doses typed on a prescription or at the till join this list by themselves — give them words for the label.</p>
        <Button className="h-10 gap-1.5 px-4 font-bold md:ml-auto" onClick={() => setEdit('new')}><Plus size={16} /> New dose</Button>
      </div>
      <DataList<Dose> label="Doses" rows={rows ?? []} rowKey={(r) => r.id} onRow={setEdit}
        empty={<EmptyState title={!rows ? 'Loading…' : 'No doses'} />}
        columns={[
          { key: 'dose', header: 'Dose', cell: (r) => <span className="font-mono">{r.dose}</span> },
          { key: 'words', header: 'On the label', cell: (r) => r.words || <Tag tone="warn">new — add what it means</Tag> },
          { key: 'day', header: 'A day', num: true, cell: (r) => (Number(r.perDay) ? qtyText(r.perDay) : 'when needed') },
          { key: 'uses', header: 'Used', num: true, cell: (r) => r.uses || '—', wide: true },
        ]} />
      {edit && <DoseSheet dose={edit === 'new' ? null : edit} onClose={() => setEdit(null)} />}
    </>
  )
}

function DoseSheet({ dose, onClose }: { dose: Dose | null; onClose: () => void }) {
  const [f, setF] = useState({ dose: dose?.dose ?? '', words: dose?.words ?? '', perDay: dose ? (Number(dose.perDay) ? dose.perDay : '0') : '' })
  const [busy, setBusy] = useState(false)
  async function save() {
    setBusy(true)
    try {
      if (dose) await updateDose(dose.id, f)
      else await createDose(f)
      toast.success('Dose saved')
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={dose ? `Dose ${dose.dose}` : 'New dose'} submit="Save dose" busy={busy} onSubmit={() => void save()}
      extra={dose && <ConfirmButton label="Delete" title={`Delete ${dose.dose}?`} confirm="Delete" body="Prescriptions keep what they say; it comes back if someone types it again."
        onConfirm={async () => { try { await deleteDose(dose.id); toast.success('Dose deleted'); onClose() } catch (e) { toast.error(errMsg(e)) } }} />}>
      <div className="grid gap-3 md:grid-cols-[10rem_1fr_7rem]">
        <Field label="Dose" hint="As written">
          <input aria-label="Dose" value={f.dose} maxLength={40} placeholder="1-0-1" className={INPUT}
            onChange={(e) => { const d = e.target.value; const per = dosesPerDay(d); setF({ ...f, dose: d, ...(!dose && per ? { perDay: String(per) } : {}) }) }} />
        </Field>
        <Field label="On the label" hint="What it means for the patient">
          <input aria-label="On the label" value={f.words} maxLength={60} placeholder="Morning · Night" onChange={(e) => setF({ ...f, words: e.target.value })} className={INPUT} />
        </Field>
        <Field label="A day" hint="0 = when needed">
          <input aria-label="Doses a day" inputMode="decimal" value={f.perDay} onChange={(e) => setF({ ...f, perDay: e.target.value.replace(/[^\d.]/g, '') })} className={INPUT} />
        </Field>
      </div>
    </FormSheet>
  )
}
