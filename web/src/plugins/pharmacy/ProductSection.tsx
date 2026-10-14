import { useQuery } from '@tanstack/react-query'
import { type ComponentProps, useEffect, useState } from 'react'
import { Field, INPUT, Toggle } from '@/components/admin/fields'
import type { ProductFormSection } from '../slots'
import { productFlagsQuery } from './api'
import type { Flags } from './types'

type Props = ComponentProps<ProductFormSection['Section']>

/** "Pharmacy" on the product form and intake New item: controlled (+ schedule) and
 * prescription-only. Hints at the till only — a sale is never stopped. A duplicate
 * or loose version starts with its source's marks. Sends only once touched (or a copy). */
export function ProductSection({ productId, sourceId, onChange }: Props) {
  const cur = useQuery(productFlagsQuery(productId || sourceId)).data
  const [edit, setEdit] = useState<Flags | null>(null)
  const f: Flags = edit ?? cur ?? { controlled: false, schedule: '', rxOnly: false }
  const send = edit !== null || (productId === 0 && !!cur && (cur.controlled || cur.rxOnly))
  const set = (p: Partial<Flags>) => setEdit({ ...f, ...p })

  useEffect(() => {
    onChange(send ? f : undefined)
    // oxlint-disable-next-line react-hooks/exhaustive-deps -- onChange is a fresh arrow each render
  }, [send, f.controlled, f.schedule, f.rxOnly])

  return (
    <fieldset className="grid gap-3 rounded-[var(--radius-sm)] border border-border p-3 md:grid-cols-2">
      <legend className="px-1 text-[12px] font-bold uppercase tracking-wide text-faint">Pharmacy</legend>
      <Toggle label="Prescription medicine (℞)" checked={f.rxOnly} onChange={(v) => set({ rxOnly: v })}
        hint="The till reminds the cashier to ask for the prescription." />
      <Toggle label="Controlled drug" checked={f.controlled} onChange={(v) => set({ controlled: v })}
        hint="Every sale goes on the controlled register with the patient." />
      {f.controlled && (
        <Field label="Schedule" hint="As the regulator lists it, e.g. Schedule II." className="md:col-span-2">
          <input aria-label="Controlled schedule" value={f.schedule} maxLength={60} onChange={(e) => set({ schedule: e.target.value })} className={INPUT} />
        </Field>
      )}
    </fieldset>
  )
}
