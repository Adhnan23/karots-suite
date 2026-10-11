import { useQuery } from '@tanstack/react-query'
import { type ComponentProps, useEffect, useState } from 'react'
import { Field } from '@/components/admin/fields'
import type { ProductFormSection } from '../slots'
import { formQuery } from './api'
import { FieldInput } from './FieldInput'
import type { FormField } from './types'

type Props = ComponentProps<ProductFormSection['Section']>
const NONE: FormField[] = [] // stable: a fresh [] would re-run the effect every render

/** "More details" on the product form and intake New item: the fields for the
 * product's category. A duplicate loads the source's values. Sends what was
 * edited, the product's own values (so a copy carries them) and any blank
 * required field (so the server refuses it) — never an untouched default, which
 * would turn "reads the default" into a value of its own. */
export function FieldsSection({ productId, sourceId, categoryId, where, onChange }: Props) {
  const fields = useQuery(formQuery(productId || sourceId, categoryId, where)).data ?? NONE
  const [edits, setEdits] = useState<Record<string, string>>({}) // by key, kept across category switches
  const valueOf = (f: FormField) => edits[f.key] ?? f.value

  useEffect(() => {
    const values: Record<string, string> = {}
    for (const f of fields) {
      const v = valueOf(f)
      if (f.key in edits || f.own || (f.required && f.type !== 'bool' && !v.trim())) values[f.key] = v
    }
    onChange({ values })
    // oxlint-disable-next-line react-hooks/exhaustive-deps -- onChange is a fresh arrow each render
  }, [fields, edits])

  if (!fields.length) return null
  return (
    <fieldset className="grid gap-3 rounded-[var(--radius-sm)] border border-border p-3 md:grid-cols-2">
      <legend className="px-1 text-[12px] font-bold uppercase tracking-wide text-faint">More details</legend>
      {fields.map((f) => {
        const input = <FieldInput field={f} value={valueOf(f)} removed={f.removed} required={f.required}
          onChange={(v) => setEdits((e) => ({ ...e, [f.key]: v }))} />
        return f.type === 'bool'
          ? <div key={f.id} className="self-end">{input}</div>
          : <Field key={f.id} label={f.required ? `${f.label} *` : f.label} hint={f.hint || undefined} className={f.type === 'textarea' ? 'md:col-span-2' : undefined}>{input}</Field>
      })}
    </fieldset>
  )
}
