import { qtyText } from '@/lib/money'
import { supplyDays } from './dosage'

export type DoseLabel = { patient?: string; drug: string; qty: number; dose: string; words: string; perDay: number; note: string; on?: Date }

/** A dose sticker's lines (printed on the label printer, no barcode): with a patient
 * their name and the date first; without one the medicine carries the date. */
export function doseLabel(d: DoseLabel): string[] {
  const date = (d.on ?? new Date()).toLocaleDateString('en-GB', { day: 'numeric', month: 'short' })
  const med = `${d.drug} × ${qtyText(d.qty)}`
  const days = supplyDays(d.qty, d.perDay)
  const last = [days ? `${days} day${days === 1 ? '' : 's'}` : '', d.note.trim()].filter(Boolean).join(' · ')
  return [
    ...(d.patient ? [`${d.patient}  ${date}`, med] : [`${med}  ${date}`]),
    d.words.trim() ? `${d.dose}  ${d.words.trim()}` : d.dose,
    ...(last ? [last] : []),
  ].filter((l) => l.trim())
}
