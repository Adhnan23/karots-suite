import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { qtyText } from '@/lib/money'
import { dosesQuery, notesQuery } from './api'
import { perDayOf } from './dosage'

type Item = { value: string; hint?: string }

/** A text box that offers a list while typing (pick or keep typing). */
function SuggestInput({ value, onChange, onPick, items, label, className, placeholder, autoFocus }: {
  value: string; onChange: (v: string) => void; onPick: (v: string) => void; items: Item[]; label: string
  className?: string; placeholder?: string; autoFocus?: boolean
}) {
  const [open, setOpen] = useState(false)
  const k = value.trim().toLowerCase()
  const hits = items.filter((d) => !k || d.value.toLowerCase().includes(k) || d.hint?.toLowerCase().includes(k)).slice(0, 8)
  const exact = hits.length === 1 && hits[0].value.toLowerCase() === k
  return (
    <div className="relative">
      <input aria-label={label} placeholder={placeholder} value={value} autoComplete="off" autoFocus={autoFocus} className={className}
        onFocus={() => setOpen(true)} onBlur={() => setTimeout(() => setOpen(false), 150)} onChange={(e) => onChange(e.target.value)} />
      {open && hits.length > 0 && !exact && (
        <div role="listbox" aria-label={`${label} — choices`}
          className="absolute left-0 top-full z-50 mt-1 grid max-h-64 min-w-56 overflow-auto rounded-[var(--radius-sm)] border border-border bg-popover p-1 text-[13px] shadow-[var(--shadow-float)]">
          {hits.map((d) => (
            <button key={d.value} type="button" role="option" aria-selected={d.value.toLowerCase() === k} onMouseDown={(e) => e.preventDefault()}
              onClick={() => { onPick(d.value); setOpen(false) }}
              className="flex items-center justify-between gap-3 rounded-md px-2.5 py-1.5 text-left hover:bg-muted">
              <span className="font-semibold">{d.value}</span>
              {d.hint && <small className="whitespace-nowrap text-faint">{d.hint}</small>}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

type Common = { value: string; label: string; className?: string; placeholder?: string; autoFocus?: boolean }

/** A dose: typed (1-0-1, tds, "1 every 8 hours") or picked from the shop's dose list —
 * onChange gets the text and its doses a day (the list's, else read from 1-0-1; 0 = unknown). */
export function DoseInput({ onChange, placeholder = '1-0-1', ...p }: Common & { onChange: (dose: string, perDay: number) => void }) {
  const doses = useQuery(dosesQuery).data
  const items = (doses ?? []).map((d) => ({ value: d.dose, hint: [d.words, Number(d.perDay) ? `${qtyText(d.perDay)} a day` : ''].filter(Boolean).join(' · ') || 'new' }))
  const set = (v: string) => onChange(v, perDayOf(v, doses))
  return <SuggestInput {...p} placeholder={placeholder} items={items} onChange={set} onPick={set} />
}

/** How to take it (After meals, Empty stomach — 30 min before food …): typed or picked. */
export function NoteInput({ onChange, placeholder = 'e.g. After meals', ...p }: Common & { onChange: (note: string) => void }) {
  const notes = useQuery(notesQuery).data ?? []
  return <SuggestInput {...p} placeholder={placeholder} items={notes.map((n) => ({ value: n }))} onChange={onChange} onPick={onChange} />
}
