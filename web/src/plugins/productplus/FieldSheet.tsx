import { useQuery } from '@tanstack/react-query'
import { AlertTriangle, X } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Field, INPUT, Toggle } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { CategoryPicker } from '@/features/admin/categories/CategoryPicker'
import { categoriesQuery } from '@/features/admin/categories/queries'
import { pathName } from '@/features/admin/categories/tree'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'
import { createField, impactQuery, updateField } from './api'
import { FieldInput } from './FieldInput'
import { type FieldDTO, type FieldRequest, type FieldType, TYPE_LABEL } from './types'

export const CHIP = 'rounded-pill border border-border bg-card px-3.5 py-2 text-[13px] font-semibold text-muted-foreground whitespace-nowrap aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground'

const WHERE = [
  { key: 'showAtTill', label: 'Till ⓘ' },
  { key: 'printOnLabel', label: 'Label' },
  { key: 'showInIntake', label: 'Stock intake' },
] as const

const blank: FieldRequest = { label: '', type: 'text', defaultValue: '', hint: '', required: false, searchable: false, showAtTill: false, printOnLabel: false, showInIntake: false, options: [], scope: [] }

const fromField = (f: FieldDTO): FieldRequest => ({
  label: f.label, type: f.type, defaultValue: f.defaultValue, hint: f.hint, required: f.required, searchable: f.searchable,
  showAtTill: f.showAtTill, printOnLabel: f.printOnLabel, showInIntake: f.showInIntake, options: f.options, scope: f.scope.map((c) => c.id),
})

const textish = (t: FieldType) => t === 'text' || t === 'textarea'

/** Add or edit a custom field. On an edit it warns before data changes: a type
 * change that deletes values needs a tick, and a new default asks whether
 * existing products keep the old one. */
export function FieldSheet({ field, onClose }: { field: FieldDTO | null; onClose: () => void }) {
  const cats = useQuery(categoriesQuery).data ?? []
  const [f, setF] = useState<FieldRequest>(() => (field ? fromField(field) : blank))
  const [opts, setOpts] = useState(() => f.options.join('\n'))
  const [defaultFor, setDefaultFor] = useState<'new' | 'all'>('new')
  const [confirmLoss, setConfirmLoss] = useState(false)
  const [busy, setBusy] = useState(false)
  const set = (p: Partial<FieldRequest>) => setF((x) => ({ ...x, ...p }))
  const options = opts.split('\n').map((o) => o.trim()).filter(Boolean)
  const typeChanged = !!field && field.type !== f.type && !(textish(field.type) && textish(f.type))
  const optionsShrank = !!field && f.type === 'select' && field.type === 'select' && field.options.some((o) => !options.includes(o))
  const im = useQuery({ ...impactQuery(field?.id ?? 0, f.type, f.type === 'select' ? options : []), enabled: typeChanged || optionsShrank }).data
  const lossy = typeChanged ? im?.lossy ?? 0 : 0
  const defaultChanged = !!field && field.defaultValue !== f.defaultValue && f.type === field.type

  async function save() {
    if (lossy > 0 && !confirmLoss) return void toast.error('Tick the box to delete the values that don’t fit')
    setBusy(true)
    try {
      const body = { ...f, options: f.type === 'select' ? options : [], defaultFor, confirmLoss: lossy > 0 && confirmLoss }
      const r = field ? await updateField(field.id, body) : await createField(body)
      toast.success(`${r.label} ${field ? 'saved' : 'added'}`)
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={field ? `Edit ${field.label}` : 'Add a field'} submit={field ? 'Save field' : 'Add field'} busy={busy} onSubmit={() => void save()}>
      <div className="grid gap-3 sm:grid-cols-2">
        <Field label="Label"><input required autoFocus maxLength={60} aria-label="Label" placeholder="Model no" value={f.label} onChange={(e) => set({ label: e.target.value })} className={INPUT} /></Field>
        <Field label="Type">
          <select aria-label="Type" value={f.type} onChange={(e) => { set({ type: e.target.value as FieldType, defaultValue: '' }); setConfirmLoss(false) }} className={cn(INPUT, 'appearance-auto')}>
            {Object.entries(TYPE_LABEL).map(([v, l]) => <option key={v} value={v}>{l}</option>)}
          </select>
        </Field>
      </div>
      {lossy > 0 && im && (
        <div role="alert" className="grid gap-2 rounded-[var(--radius-sm)] border border-danger bg-[color-mix(in_srgb,var(--danger)_10%,transparent)] p-3 text-[13px]">
          <p className="flex gap-2 font-semibold text-danger"><AlertTriangle size={16} className="mt-0.5 flex-none" />
            {im.lossy === 1 ? `1 of ${im.total} products’ values doesn’t` : `${im.lossy} of ${im.total} products’ values don’t`} fit {TYPE_LABEL[f.type]} and will be deleted. This can’t be undone.</p>
          <label className="flex items-center gap-2"><input type="checkbox" className="size-4 accent-[var(--danger)]" checked={confirmLoss} onChange={(e) => setConfirmLoss(e.target.checked)} />
            {im.lossy === 1 ? 'Delete that value' : `Delete those ${im.lossy} values`}</label>
        </div>
      )}
      {f.type === 'select' && (
        <Field label="Options" hint={optionsShrank && im && im.lossy > 0 ? <span className="text-warning">{im.lossy} products keep a removed option — it shows as “(removed)” until changed.</span> : 'One per line'}>
          <textarea aria-label="Options" rows={4} value={opts} onChange={(e) => setOpts(e.target.value)} placeholder={'Grade A\nGrade B'} className={cn(INPUT, 'h-auto py-2 font-mono text-sm')} />
        </Field>
      )}
      <div className="grid gap-3 sm:grid-cols-2">
        <Field label="Default" hint="Products with no value of their own read this.">
          <FieldInput compact field={{ label: 'Default', type: f.type, options }} value={f.defaultValue} onChange={(defaultValue) => set({ defaultValue })} />
        </Field>
        <Field label="Help text" hint="Optional — shown under the field on the product form.">
          <input aria-label="Help text" value={f.hint} onChange={(e) => set({ hint: e.target.value })} className={INPUT} />
        </Field>
      </div>
      {defaultChanged && (
        <fieldset className="grid gap-2 rounded-[var(--radius-sm)] border border-border p-3 text-[13px]">
          <legend className="px-1 font-bold">The default changed — existing products?</legend>
          <label className="flex items-start gap-2"><input type="radio" name="default-for" className="mt-0.5 accent-[var(--brand)]" checked={defaultFor === 'new'} onChange={() => setDefaultFor('new')} />
            <span><b>Only new products.</b> Existing products keep {field.defaultValue ? `“${field.defaultValue}”` : 'what they show now'}.</span></label>
          <label className="flex items-start gap-2"><input type="radio" name="default-for" className="mt-0.5 accent-[var(--brand)]" checked={defaultFor === 'all'} onChange={() => setDefaultFor('all')} />
            <span><b>Change existing products too.</b> {field.usingDefault} product{field.usingDefault === 1 ? '' : 's'} with no value of their own will show {f.defaultValue ? `“${f.defaultValue}”` : 'blank'}.</span></label>
        </fieldset>
      )}
      <div className="grid gap-2 sm:grid-cols-2">
        <Toggle label="Required" hint="The product form won’t save without it." checked={f.required} onChange={(required) => set({ required })} />
        <Toggle label="Searchable" hint="Find products by it at the till and in lists." checked={f.searchable} onChange={(searchable) => set({ searchable })} />
      </div>
      <Field label="Also shows on">
        <div className="flex flex-wrap gap-1.5">
          {WHERE.map((w) => (
            <button key={w.key} type="button" aria-pressed={f[w.key]} className={CHIP} onClick={() => set({ [w.key]: !f[w.key] })}>{w.label}</button>
          ))}
        </div>
      </Field>
      <Field label="For" hint="Pick categories to limit the field to them and their subcategories.">
        <div className="flex flex-wrap items-center gap-1.5">
          {f.scope.length === 0 && <span className="rounded-pill bg-muted px-3 py-1.5 text-[13px] font-semibold">All products</span>}
          {f.scope.map((id) => (
            <span key={id} className="inline-flex items-center gap-1 rounded-pill bg-brand-soft py-1 pr-1 pl-3 text-[13px] font-semibold">
              {pathName(cats, id) || field?.scope.find((c) => c.id === id)?.name}
              <button type="button" aria-label={`Remove ${pathName(cats, id)}`} onClick={() => set({ scope: f.scope.filter((x) => x !== id) })} className="grid size-6 place-items-center rounded-full hover:bg-muted"><X size={13} /></button>
            </span>
          ))}
        </div>
        <CategoryPicker label="Add a category" noneLabel="Add a category…" value={0} exclude={new Set(f.scope)}
          onChange={(id) => id && !f.scope.includes(id) && set({ scope: [...f.scope, id] })} />
      </Field>
    </FormSheet>
  )
}
