import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { dosesQuery } from './api'
import { perDayOf } from './dosage'

/** A dose box: type it (1-0-1, tds, "1 every 8 hours") or pick one the shop has used —
 * onChange gets the text and its doses a day (remembered, else read from 1-0-1; 0 = unknown). */
export function DoseInput({ value, onChange, label, className, placeholder = '1-0-1', autoFocus }: {
  value: string; onChange: (dose: string, perDay: number) => void; label: string; className?: string; placeholder?: string; autoFocus?: boolean
}) {
  const doses = useQuery(dosesQuery).data
  const [open, setOpen] = useState(false)
  const k = value.trim().toLowerCase()
  const hits = (doses ?? []).filter((d) => !k || d.dose.toLowerCase().includes(k)).slice(0, 8)
  return (
    <div className="relative">
      <input aria-label={label} placeholder={placeholder} value={value} autoComplete="off" autoFocus={autoFocus} className={className}
        onFocus={() => setOpen(true)} onBlur={() => setTimeout(() => setOpen(false), 150)}
        onChange={(e) => onChange(e.target.value, perDayOf(e.target.value, doses))} />
      {open && hits.length > 0 && !(hits.length === 1 && hits[0].dose.toLowerCase() === k) && (
        <div role="listbox" aria-label={`${label} — used before`}
          className="absolute left-0 top-full z-50 mt-1 grid max-h-60 min-w-44 overflow-auto rounded-[var(--radius-sm)] border border-border bg-popover p-1 text-[13px] shadow-[var(--shadow-float)]">
          {hits.map((d) => (
            <button key={d.dose} type="button" role="option" aria-selected={d.dose.toLowerCase() === k} onMouseDown={(e) => e.preventDefault()}
              onClick={() => { onChange(d.dose, Number(d.perDay)); setOpen(false) }}
              className={cn('flex items-center justify-between gap-3 rounded-md px-2.5 py-1.5 text-left hover:bg-muted')}>
              <span className="font-semibold">{d.dose}</span>
              <small className="whitespace-nowrap text-faint">{Number(d.perDay) ? `${qtyText(d.perDay)} a day` : 'a day?'}</small>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
