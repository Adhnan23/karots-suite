import type { TillCopy } from '@/api/types.gen'
import { LKR_NOTES, notesFor } from '@/features/till/cart/totals'
import { num, r2 } from '@/lib/money'

/** The change to hand back, note by note then coin by coin, from the shop's own
 * notes and coins (Sri Lanka's notes until the shop lists them). The till's
 * notesFor, run over the notes first, so a coin never stands in for a note. */
export function changeNotes(change: number, copy: TillCopy): { value: number; qty: number; isNote: boolean }[] {
  const ds = copy.denominations ?? []
  const notes = ds.length ? ds.filter((d) => d.isNote).map((d) => num(d.value)) : LKR_NOTES
  const coins = ds.filter((d) => !d.isNote).map((d) => num(d.value))
  const byNote = notesFor(change, notes)
  const left = r2(change - byNote.reduce((s, n) => s + n.value * n.qty, 0))
  return [...byNote.map((n) => ({ ...n, isNote: true })), ...notesFor(left, coins).map((n) => ({ ...n, isNote: false }))]
}
