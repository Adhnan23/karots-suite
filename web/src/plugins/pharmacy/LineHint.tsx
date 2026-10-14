import { useQuery } from '@tanstack/react-query'
import type { CartLine } from '@/features/till/cart/types'
import type { TillApi } from '../slots'
import { tillInfoQuery } from './api'
import { salePatient } from './sale'

/** Under a cart line: which script it is for, and reminders that never block — a
 * controlled drug wants the patient (💊) on the sale, a prescription medicine the script. */
export function LineHint({ till, line }: { till: TillApi; line: CartLine }) {
  const f = useQuery(tillInfoQuery([line.productId])).data?.[line.productId]
  const tag = line.plugin?.pharmacy as { drug?: string } | undefined
  const patient = salePatient(till)
  const notes: { text: string; warn?: boolean }[] = []
  if (tag?.drug) notes.push({ text: `💊 For ${patient?.name ?? 'the patient'}’s ${tag.drug}` })
  if (f?.controlled && !patient) notes.push({ text: '⚠ Controlled — add the patient (💊)', warn: true })
  else if (f?.controlled) notes.push({ text: `⚠ Controlled${f.schedule ? ` · ${f.schedule}` : ''} — on the register for ${patient?.name}` })
  if (f?.rxOnly && !tag) notes.push({ text: '℞ Prescription medicine' })
  if (!notes.length) return null
  return (
    <div className="mt-1 flex flex-wrap gap-1 text-[11.5px]">
      {notes.map((n) => (
        <span key={n.text} className={n.warn ? 'rounded-md bg-[color-mix(in_srgb,var(--warning)_14%,transparent)] px-1.5 py-0.5 font-semibold text-warning' : 'rounded-md bg-muted px-1.5 py-0.5 text-muted-foreground'}>
          {n.text}
        </span>
      ))}
    </div>
  )
}
