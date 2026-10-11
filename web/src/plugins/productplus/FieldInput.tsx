import { INPUT, Toggle } from '@/components/admin/fields'
import { cn } from '@/lib/utils'
import type { FieldType } from './types'

type Props = {
  field: { label: string; type: FieldType; options: string[]; hint?: string }
  value: string
  onChange: (v: string) => void
  /** the stored value is a dropdown option since removed: keep offering it */
  removed?: boolean
  required?: boolean
  /** a grid cell: no Toggle card, placeholder shows the default */
  compact?: boolean
  placeholder?: string
  invalid?: boolean
}

/** One custom field's control, the same on the product form, intake and Fill in. */
export function FieldInput({ field, value, onChange, removed, required, compact, placeholder, invalid }: Props) {
  const cls = cn(INPUT, compact && 'h-9', invalid && 'border-danger')
  switch (field.type) {
    case 'bool':
      return compact
        ? <input type="checkbox" aria-label={field.label} checked={value === '1'} onChange={(e) => onChange(e.target.checked ? '1' : '')} className="size-5 accent-[var(--brand)]" />
        : <Toggle label={field.label} hint={field.hint || undefined} checked={value === '1'} onChange={(on) => onChange(on ? '1' : '')} />
    case 'select':
      return (
        <select aria-label={field.label} required={required} value={value} onChange={(e) => onChange(e.target.value)} className={cn(cls, 'appearance-auto')}>
          <option value="">{placeholder ? `— ${placeholder}` : '—'}</option>
          {removed && value && <option value={value}>{value} (removed)</option>}
          {field.options.map((o) => <option key={o} value={o}>{o}</option>)}
        </select>
      )
    case 'textarea':
      return compact
        ? <input aria-label={field.label} value={value} placeholder={placeholder} onChange={(e) => onChange(e.target.value)} className={cls} />
        : <textarea aria-label={field.label} required={required} rows={3} value={value} onChange={(e) => onChange(e.target.value)} className={cn(cls, 'h-auto py-2')} />
    case 'number':
      return <input aria-label={field.label} required={required} inputMode="decimal" value={value} placeholder={placeholder} onChange={(e) => onChange(e.target.value)} className={cn(cls, 'tabular-nums')} />
    case 'date':
      return <input aria-label={field.label} required={required} type="date" value={value} onChange={(e) => onChange(e.target.value)} className={cls} />
    default:
      return <input aria-label={field.label} required={required} value={value} placeholder={placeholder} onChange={(e) => onChange(e.target.value)} className={cls} />
  }
}
