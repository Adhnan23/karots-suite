import { useQuery } from '@tanstack/react-query'
import { X } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import type { CartLine } from '@/features/till/cart/types'
import { configQuery } from '@/features/till/api'
import { money } from '@/lib/money'
import type { TillApi } from '@/plugins/slots'
import { settingsQuery, tillCombosQuery } from './api'
import { findHints, findPartials } from './match'
import { setPrice } from './pricing'
import { addCombo } from './till'

/** This sale's notes (TillApi sale meta, so New sale clears them and hold keeps them):
 * combos the cashier waved away (✕) and combos they broke apart (auto apply leaves
 * those to the hint). */
type Notes = { hidden?: number[]; broken?: number[] }
const notesOf = (till: TillApi) => (till.saleMeta.combos as Notes | undefined) ?? {}
const note = (till: TillApi, k: keyof Notes, id: number) => {
  const n = notesOf(till)
  if (!n[k]?.includes(id)) till.setSaleMeta('combos', { ...n, [k]: [...(n[k] ?? []), id] })
}
const hide = (till: TillApi, id: number) => note(till, 'hidden', id)

const run = async (p: Promise<string | null>, done: string) => {
  const err = await p
  if (err) toast.error(err)
  else toast.success(done)
}

/** The combos rung as groups in the cart. */
const grouped = (lines: CartLine[]) =>
  new Set(lines.filter((l) => l.group).map((l) => Number((l.plugin?.combos as { comboId?: number } | undefined)?.comboId) || 0).filter(Boolean))

let applying = false

/** "🎁 Milk + Cookies ×2 — save Rs 80 · Apply": the cart holds full sets as plain lines.
 * With the shop's auto apply on, the till applies it itself; a combo the cashier
 * broke apart is left to the hint for the rest of the sale. Mounted even on an empty
 * cart, which clears the sale's notes. */
export function CartHint({ till, lines }: { till: TillApi; lines: CartLine[] }) {
  const sym = useQuery(configQuery).data?.symbol ?? ''
  const combos = useQuery(tillCombosQuery).data ?? []
  const auto = useQuery(settingsQuery).data?.autoApply ?? false
  const { hidden = [], broken = [] } = notesOf(till)
  const hints = findHints(combos.filter((c) => !hidden.includes(c.id)), lines)
  const was = useRef(grouped(lines))
  useEffect(() => {
    if (!lines.length && till.saleMeta.combos) till.setSaleMeta('combos', undefined)
    const now = grouped(lines)
    // A combo gone while its items stay in the cart was broken apart (not removed).
    for (const id of was.current) {
      const c = combos.find((x) => x.id === id)
      if (!now.has(id) && c && lines.some((l) => !l.group && c.members.some((m) => m.productId === l.productId))) note(till, 'broken', id)
    }
    was.current = now
  }, [lines])
  const next = auto ? hints.find((h) => !broken.includes(h.combo.id)) : undefined
  useEffect(() => {
    if (!next || applying) return
    applying = true
    void addCombo(till, next.combo, next.sets, next.take).then((err) => {
      applying = false
      if (err) { hide(till, next.combo.id); toast.error(err) } else toast.success(`${next.combo.name} applied`)
    })
  })
  const shown = auto ? hints.filter((h) => broken.includes(h.combo.id)) : hints
  if (!shown.length) return null
  return (
    <>
      {shown.map((h) => (
        <div key={h.combo.id} className="grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-2 rounded-[var(--radius-sm)] border-[1.5px] border-dashed border-brand bg-brand-soft px-2.5 py-2">
          <span className="min-w-0">
            <b className="block text-[13.5px] leading-snug">🎁 {h.combo.name}{h.sets > 1 && ` ×${h.sets}`}</b>
            <span className="text-[12px] font-semibold text-success tabular-nums">save {money(sym, (setPrice(h.combo, till.saleType).off * h.sets) / 100)}</span>
          </span>
          <Button className="h-8 px-3.5 font-bold" onClick={() => void run(addCombo(till, h.combo, h.sets, h.take), `${h.combo.name} applied`)}>Apply</Button>
          <button type="button" aria-label={`Not now: ${h.combo.name}`} onClick={() => hide(till, h.combo.id)} className="grid size-7 place-items-center text-faint hover:text-foreground"><X size={15} /></button>
        </div>
      ))}
    </>
  )
}

/** Under a member line when the cart holds part of a combo: tap adds the rest as one set. */
export function LineHint({ till, line, lines }: { till: TillApi; line: CartLine; lines: CartLine[] }) {
  const sym = useQuery(configQuery).data?.symbol ?? ''
  const combos = useQuery(tillCombosQuery).data ?? []
  const hidden = notesOf(till).hidden ?? []
  const p = findPartials(combos.filter((c) => !hidden.includes(c.id)), lines).find((x) => x.take.some((t) => t.key === line.key))
  if (!p) return null
  return (
    <button type="button" onClick={() => void run(addCombo(till, p.combo, 1, p.take), `${p.combo.name} added`)}
      className="mt-1 max-w-full rounded-md border border-dashed border-brand bg-brand-soft px-2 py-0.5 text-left text-[11.5px] font-semibold hover:border-solid">
      🎁 {p.combo.name} · add {p.missing.join(', ')} · save {money(sym, setPrice(p.combo, till.saleType).off / 100)}
    </button>
  )
}
