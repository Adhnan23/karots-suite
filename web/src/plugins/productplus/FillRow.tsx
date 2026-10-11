import { cn } from '@/lib/utils'
import { FieldInput } from './FieldInput'
import { cellKey, type Edits, type FieldDTO, type GridRow } from './types'

/** One product × field cell: the stored value, or the default as a placeholder
 * (null = reads it); a field outside the product's scope shows a dash. */
function Cell({ r, f, e }: { r: GridRow; f: FieldDTO; e: Edits }) {
  if (!(f.id in r.values)) return <span className="text-faint" title="Not for this product’s category">—</span>
  const k = cellKey(r.id, f.id)
  const stored = r.values[f.id]
  const value = e.dirty[k] ?? (stored ?? (f.type === 'bool' ? f.defaultValue : ''))
  const err = e.errors[k]
  return (
    <span className="flex flex-col gap-1">
      <span className={cn('rounded-[var(--radius-sm)]', k in e.dirty && 'ring-2 ring-selected', f.type === 'bool' && 'w-fit p-1')}>
        <FieldInput compact field={{ ...f, label: `${f.label} for ${r.name}` }} value={value} invalid={!!err}
          placeholder={stored === null && f.type !== 'bool' ? f.defaultValue : undefined}
          removed={f.type === 'select' && !!value && !f.options.includes(value)} onChange={(v) => e.edit(r.id, f.id, v)} />
      </span>
      {err && <small role="alert" className="text-[12px] font-semibold text-danger">{err}</small>}
    </span>
  )
}

const Name = ({ r }: { r: GridRow }) => (
  <span className="flex flex-col">
    <b className="font-semibold">{r.name}</b>
    <small className="text-faint">{r.categoryName || 'No category'}</small>
  </span>
)

/** A Fill in row: a table row from 768px, a card (one field) below it. */
export function FillRow({ r, cols, e, card }: { r: GridRow; cols: FieldDTO[]; e: Edits; card?: boolean }) {
  if (card) return (
    <li className="flex flex-col gap-2 px-4 py-3">
      <Name r={r} />
      {cols.map((f) => <Cell key={f.id} r={r} f={f} e={e} />)}
    </li>
  )
  return (
    <tr className="border-b border-border last:border-0">
      <td className="px-4 py-2.5"><Name r={r} /></td>
      {cols.map((f) => <td key={f.id} className="min-w-40 px-4 py-2.5 align-top"><Cell r={r} f={f} e={e} /></td>)}
    </tr>
  )
}
