import { useQuery } from '@tanstack/react-query'
import { type ComponentProps, useEffect, useState } from 'react'
import { Field, INPUT, Toggle } from '@/components/admin/fields'
import type { ProductFormSection } from '../slots'
import { productFlagsQuery } from './api'
import { DoseInput, NoteInput } from './DoseInput'
import type { Flags } from './types'

type Props = ComponentProps<ProductFormSection['Section']>

/** "Pharmacy" on the product form and intake New item: the usual dose, prescription-only
 * and — only for a prescription medicine — controlled (+ schedule). Hints at the till only. A duplicate
 * or loose version starts with its source's marks. Sends only once touched (or a copy). */
export function ProductSection({ productId, sourceId, onChange }: Props) {
  const cur = useQuery(productFlagsQuery(productId || sourceId)).data
  const [edit, setEdit] = useState<Flags | null>(null)
  const f: Flags = edit ?? cur ?? { controlled: false, schedule: '', rxOnly: false, dose: '', note: '' }
  const send = edit !== null || (productId === 0 && !!cur && (cur.controlled || cur.rxOnly || !!cur.dose || !!cur.note))
  const set = (p: Partial<Flags>) => setEdit({ ...f, ...p })

  useEffect(() => {
    onChange(send ? f : undefined)
    // oxlint-disable-next-line react-hooks/exhaustive-deps -- onChange is a fresh arrow each render
  }, [send, f.controlled, f.schedule, f.rxOnly, f.dose, f.note])

  return (
    <fieldset className="grid gap-3 rounded-[var(--radius-sm)] border border-border p-3 md:grid-cols-2">
      <legend className="px-1 text-[12px] font-bold uppercase tracking-wide text-faint">Pharmacy</legend>
      <Field label="Usual dose" hint="Fills prescriptions, the till's Days… and the dose label — always changeable.">
        <DoseInput label="Usual dose" value={f.dose} onChange={(d) => set({ dose: d })} placeholder="e.g. 1-0-1 — optional" className={INPUT} />
      </Field>
      <Field label="How to take it" hint="Printed on the dose label, e.g. Empty stomach, 30 min before food.">
        <NoteInput label="How to take it" value={f.note} onChange={(v) => set({ note: v })} placeholder="e.g. After meals — optional" className={INPUT} />
      </Field>
      <Toggle className="md:col-start-1" label="Prescription medicine (℞)" checked={f.rxOnly} onChange={(v) => set({ rxOnly: v, ...(!v && { controlled: false }) })}
        hint="The till reminds the cashier to ask for the prescription." />
      {f.rxOnly && (
        <Toggle label="Controlled drug" checked={f.controlled} onChange={(v) => set({ controlled: v })} className="md:col-start-2"
          hint="Every sale goes on the controlled register with the patient." />
      )}
      {f.rxOnly && f.controlled && (
        <Field label="Schedule" hint="As the regulator lists it, e.g. Schedule II." className="md:col-span-2">
          <input aria-label="Controlled schedule" value={f.schedule} maxLength={60} onChange={(e) => set({ schedule: e.target.value })} className={INPUT} />
        </Field>
      )}
    </fieldset>
  )
}
