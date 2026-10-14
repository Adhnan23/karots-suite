import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import type { CartLine } from '@/features/till/cart/types'
import { FIELD } from '@/features/till/ui'
import { cn } from '@/lib/utils'
import type { TillApi } from '../slots'
import { tillInfoQuery } from './api'
import { dosesPerDay, tabletsFor } from './dosage'
import { salePatient } from './sale'

/** Under a cart line: which script it is for, reminders that never block (a controlled
 * drug wants the patient (💊) on the sale, a prescription medicine the script), and on a
 * medicine Days… — "1-0-1 × 2 days" sets the line to 4. */
export function LineHint({ till, line }: { till: TillApi; line: CartLine }) {
  const f = useQuery(tillInfoQuery([line.productId])).data?.[line.productId]
  const [days, setDays] = useState(false)
  const tag = line.plugin?.pharmacy as { drug?: string } | undefined
  const patient = salePatient(till)
  const notes: { text: string; warn?: boolean }[] = []
  if (tag?.drug) notes.push({ text: `💊 For ${patient?.name ?? 'the patient'}’s ${tag.drug}` })
  if (f?.controlled && !patient) notes.push({ text: '⚠ Controlled — add the patient (💊)', warn: true })
  else if (f?.controlled) notes.push({ text: `⚠ Controlled${f.schedule ? ` · ${f.schedule}` : ''} — on the register for ${patient?.name}` })
  if (f?.rxOnly && !tag) notes.push({ text: '℞ Prescription medicine' })
  if (!notes.length && !f?.medicine) return null
  return (
    <div className="mt-1 grid gap-1">
      <div className="flex flex-wrap items-center gap-1 text-[11.5px]">
        {notes.map((n) => (
          <span key={n.text} className={n.warn ? 'rounded-md bg-[color-mix(in_srgb,var(--warning)_14%,transparent)] px-1.5 py-0.5 font-semibold text-warning' : 'rounded-md bg-muted px-1.5 py-0.5 text-muted-foreground'}>
            {n.text}
          </span>
        ))}
        {f?.medicine && !days && (
          <button type="button" onClick={() => setDays(true)} className="rounded-md border border-dashed border-border px-1.5 py-0.5 font-semibold text-muted-foreground hover:border-brand hover:text-foreground">
            Days…
          </button>
        )}
      </div>
      {days && <DaysForm onSet={(n) => { till.setLineQty(line.key, n); setDays(false) }} onCancel={() => setDays(false)} />}
    </div>
  )
}

/** "1-0-1 × 2 days = 4" → Set 4 on the line. */
function DaysForm({ onSet, onCancel }: { onSet: (n: number) => void; onCancel: () => void }) {
  const [dose, setDose] = useState('1-0-1')
  const [days, setDays] = useState('')
  const n = tabletsFor(dosesPerDay(dose), Number(days) || 0)
  return (
    <form className="flex flex-wrap items-center gap-1.5 text-[12.5px]" onSubmit={(e) => { e.preventDefault(); if (n > 0) onSet(n) }}>
      <input aria-label="Dose" value={dose} onChange={(e) => setDose(e.target.value)} className={cn(FIELD, 'h-8 w-20 px-2 text-center text-[13px]')} />
      <span>×</span>
      <input autoFocus aria-label="Days" inputMode="numeric" placeholder="days" value={days} onChange={(e) => setDays(e.target.value.replace(/\D/g, ''))}
        className={cn(FIELD, 'h-8 w-16 px-2 text-center text-[13px]')} />
      <span>days = <b>{n || '—'}</b></span>
      <button type="submit" disabled={n <= 0} className="rounded-md bg-brand px-2.5 py-1 font-bold text-brand-foreground disabled:opacity-50">Set {n || ''}</button>
      <button type="button" onClick={onCancel} className="px-1 text-muted-foreground hover:text-foreground">Cancel</button>
    </form>
  )
}
