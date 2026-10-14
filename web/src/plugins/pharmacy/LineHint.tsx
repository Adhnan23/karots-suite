import { useQuery } from '@tanstack/react-query'
import { Tag as TagIcon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { errMsg } from '@/features/till/api'
import type { CartLine } from '@/features/till/cart/types'
import { FIELD } from '@/features/till/ui'
import { cn } from '@/lib/utils'
import type { TillApi } from '../slots'
import { dosesQuery, printTextLabel, tillInfoQuery } from './api'
import { perDayOf, supplyDays, tabletsFor } from './dosage'
import { DoseInput, NoteInput } from './DoseInput'
import { doseLabel } from './label'
import { salePatient } from './sale'

/** Dose and instruction set with Days… per cart line, kept on the sale (TillApi.saleMeta). */
type LineDoses = Record<string, { dose: string; note: string }>
const CHIP = 'rounded-md border border-dashed border-border px-1.5 py-0.5 font-semibold text-muted-foreground hover:border-brand hover:text-foreground'

/** Under a cart line: which script it is for, reminders that never block (a controlled
 * drug wants the patient (💊) on the sale, a prescription medicine the script), and on a
 * medicine Days… ("1-0-1 × 2 days" sets the line to 4) and 🏷 Label (a dose sticker). */
export function LineHint({ till, line }: { till: TillApi; line: CartLine }) {
  const f = useQuery(tillInfoQuery([line.productId])).data?.[line.productId]
  const [open, setOpen] = useState<'days' | 'label' | null>(null)
  const tag = line.plugin?.pharmacy as { drug?: string; dose?: string; note?: string } | undefined
  const set = (till.saleMeta.pharmacyLines as LineDoses | undefined)?.[line.key]
  // The line's dose: the prescription's, else what Days… set, else the product's usual one.
  const dose = tag?.dose || set?.dose || f?.dose || ''
  const note = tag?.note || set?.note || f?.note || ''
  const patient = salePatient(till)
  const notes: { text: string; warn?: boolean }[] = []
  if (tag?.drug) notes.push({ text: `💊 For ${patient?.name ?? 'the patient'}’s ${tag.drug}` })
  if (f?.controlled && !patient) notes.push({ text: '⚠ Controlled — add the patient (💊)', warn: true })
  else if (f?.controlled) notes.push({ text: `⚠ Controlled${f.schedule ? ` · ${f.schedule}` : ''} — on the register for ${patient?.name}` })
  if (f?.rxOnly && !tag) notes.push({ text: '℞ Prescription medicine' })
  if (!notes.length && !f?.medicine && !tag) return null
  return (
    <div className="mt-1 grid gap-1">
      <div className="flex flex-wrap items-center gap-1 text-[11.5px]">
        {notes.map((n) => (
          <span key={n.text} className={n.warn ? 'rounded-md bg-[color-mix(in_srgb,var(--warning)_14%,transparent)] px-1.5 py-0.5 font-semibold text-warning' : 'rounded-md bg-muted px-1.5 py-0.5 text-muted-foreground'}>
            {n.text}
          </span>
        ))}
        {(f?.medicine || tag) && !open && <button type="button" onClick={() => setOpen('days')} className={CHIP}>Days…</button>}
        {dose && !open && <button type="button" onClick={() => setOpen('label')} className={cn(CHIP, 'inline-flex items-center gap-1')}><TagIcon size={11} /> Label</button>}
      </div>
      {open === 'days' && (
        <DaysForm qty={line.qty} usual={dose} usualNote={note} onCancel={() => setOpen(null)} onSet={(n, d, nt) => {
          till.setLineQty(line.key, n)
          till.setSaleMeta('pharmacyLines', { ...(till.saleMeta.pharmacyLines as LineDoses | undefined), [line.key]: { dose: d, note: nt } })
          setOpen(null)
        }} />
      )}
      {open === 'label' && <LabelForm lines={{ patient: patient?.name, drug: line.name, qty: line.qty, dose, note }} onDone={() => setOpen(null)} />}
    </div>
  )
}

/** "1-0-1 × 2 days = 4" (+ how to take it) → Set 4 on the line; the dose and note go on its label. */
function DaysForm({ qty, usual, usualNote, onSet, onCancel }: { qty: number; usual: string; usualNote: string; onSet: (n: number, dose: string, note: string) => void; onCancel: () => void }) {
  const doses = useQuery(dosesQuery).data
  const [dose, setDose] = useState(usual || '1-0-1')
  const [per, setPer] = useState<number | null>(null) // null = read from the dose (the list first)
  const [note, setNote] = useState(usualNote)
  const perDay = per ?? perDayOf(dose, doses)
  // Days start from what the line already holds (6 at 1-1-1 = 2 days), when that is whole days.
  const held = supplyDays(qty, perDay)
  const [typed, setDays] = useState<string | null>(null)
  const days = typed ?? (held > 0 && tabletsFor(perDay, held) === qty ? String(held) : '')
  const n = tabletsFor(perDay, Number(days) || 0)
  const box = cn(FIELD, 'h-8 px-2 text-center text-[13px]')
  return (
    <form className="flex flex-wrap items-center gap-1.5 text-[12.5px]" onSubmit={(e) => { e.preventDefault(); if (n > 0) onSet(n, dose.trim(), note.trim()) }}>
      <DoseInput label="Dose" value={dose} onChange={(d, p) => { setDose(d); setPer(p) }} className={cn(box, 'w-24')} />
      <span>×</span>
      <input autoFocus aria-label="Days" inputMode="numeric" placeholder="days" value={days} onChange={(e) => setDays(e.target.value.replace(/\D/g, ''))} className={cn(box, 'w-16')} />
      <span>days = <b>{n || '—'}</b></span>
      <NoteInput label="How to take it" value={note} onChange={setNote} placeholder="How to take (optional)" className={cn(box, 'w-48 text-left')} />
      <button type="submit" disabled={n <= 0} className="rounded-md bg-brand px-2.5 py-1 font-bold text-brand-foreground disabled:opacity-50">Set {n || ''}</button>
      <button type="button" onClick={onCancel} className="px-1 text-muted-foreground hover:text-foreground">Cancel</button>
    </form>
  )
}

/** The dose sticker as it will print (label printer, no barcode). */
function LabelForm({ lines: l, onDone }: { lines: { patient?: string; drug: string; qty: number; dose: string; note: string }; onDone: () => void }) {
  const doses = useQuery(dosesQuery).data
  const d = doses?.find((x) => x.dose.trim().toLowerCase() === l.dose.trim().toLowerCase())
  const text = doseLabel({ ...l, words: d?.words ?? '', perDay: perDayOf(l.dose, doses) })
  const [busy, setBusy] = useState(false)
  async function print() {
    setBusy(true)
    try {
      await printTextLabel(text)
      toast.success('Dose label printed')
      onDone()
    } catch (e) {
      toast.error(errMsg(e, 'Could not print the label'))
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className="flex flex-wrap items-end gap-2">
      <pre aria-label="Dose label" className="rounded-md border border-border bg-paper px-2.5 py-1.5 font-mono text-[11.5px] leading-snug text-[var(--ink)]">{text.join('\n')}</pre>
      <button type="button" disabled={busy} onClick={() => void print()} className="rounded-md bg-brand px-2.5 py-1 text-[12.5px] font-bold text-brand-foreground disabled:opacity-50">Print label</button>
      <button type="button" onClick={onDone} className="px-1 text-[12.5px] text-muted-foreground hover:text-foreground">Close</button>
    </div>
  )
}
