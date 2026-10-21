import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdjustRequest } from '@/api/types.gen'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { configQuery, errMsg } from '@/features/till/api'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { lotsQuery } from './queries'

const REASONS = ['Count correction', 'Found extra', 'Missing', 'Opening stock']

/** Set a product's on-hand to what's really there (karots-pos Stock → Adjust).
 * A reduction on a product with several lots asks which lot the units left. */
export function AdjustForm({ productId, onClose }: { productId: number; onClose: () => void }) {
  const p = useQuery(lotsQuery(productId)).data
  const sym = useSuspenseQuery(configQuery).data.symbol
  const qc = useQueryClient()
  const [count, setCount] = useState('')
  const [note, setNote] = useState('')
  const [lot, setLot] = useState(0)
  const [busy, setBusy] = useState(false)
  if (!p) return null
  const cur = num(p.stockQty)
  const delta = count === '' ? 0 : num(count) - cur
  const oldest = p.lots[0]?.id ?? 0
  const chosen = p.lots.find((l) => l.id === (lot || oldest))
  const worth = Math.abs(delta) * num(delta < 0 ? chosen?.costPrice : p.lots.at(-1)?.costPrice)

  async function save() {
    setBusy(true)
    try {
      const body: AdjustRequest = { productId, newQuantity: count, note, batchId: delta < 0 && p!.lots.length > 1 ? lot || oldest : 0 }
      await api('/admin/stock/adjust', { json: body })
      toast.success(`${p!.name}: ${qtyText(cur)} → ${qtyText(count)} ${p!.unitAbbr}`)
      void qc.invalidateQueries({ queryKey: ['admin'] })
      void qc.invalidateQueries({ queryKey: ['till'] })
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Adjust ${p.name}`} submit="Save count" busy={busy || count === '' || delta === 0} onSubmit={save}>
      <div className="grid grid-cols-2 gap-3">
        <div className="rounded-[var(--radius-sm)] bg-muted px-3 py-2.5">
          <small className="text-muted-foreground">On the books</small>
          <b className="block font-display text-2xl tabular-nums">{qtyText(cur)} <small className="text-sm font-medium text-faint">{p.unitAbbr}</small></b>
        </div>
        <Field label="Actually there">
          <input autoFocus required inputMode="decimal" aria-label="Actually there" value={count} onChange={(e) => setCount(e.target.value.replace(/[^\d.]/g, ''))}
            className={cn(INPUT, 'h-[52px] font-display text-2xl font-bold tabular-nums')} placeholder={qtyText(cur)} />
        </Field>
      </div>
      {delta !== 0 && (
        <p className={cn('rounded-[var(--radius-sm)] px-3 py-2 text-sm font-semibold', delta > 0 ? 'bg-[color-mix(in_srgb,var(--success)_14%,transparent)] text-success' : 'bg-[color-mix(in_srgb,var(--danger)_14%,transparent)] text-danger')}>
          {delta > 0 ? '↓ ' : '↑ '}{delta > 0 ? '+' : '−'}{qtyText(Math.abs(delta))} {p.unitAbbr} · worth {money(sym, worth)} at cost
        </p>
      )}
      {delta < 0 && p.lots.length > 1 && (
        <fieldset className="flex flex-col gap-1.5">
          <legend className="mb-1.5 text-[13px] font-semibold text-muted-foreground">Which lot are they missing from?</legend>
          {p.lots.map((l, i) => (
            <label key={l.id} className="flex cursor-pointer items-center gap-3 rounded-[var(--radius-sm)] border border-border px-3 py-2 has-checked:border-selected has-checked:bg-muted">
              <input type="radio" name="adj-lot" checked={(lot || oldest) === l.id} onChange={() => setLot(l.id)} />
              <span className="flex-1 text-sm">
                <b>{qtyText(l.qtyRemaining)} {p.unitAbbr}</b> at {money(sym, l.sellingPrice || p.sellingPrice)}
                <small className="block text-faint">{i === 0 ? 'oldest · ' : ''}{l.expiryDate ? `expires ${l.expiryDate} · ` : ''}cost {money(sym, l.costPrice)}</small>
              </span>
            </label>
          ))}
        </fieldset>
      )}
      <Field label="Why">
        <input aria-label="Note" value={note} onChange={(e) => setNote(e.target.value)} className={INPUT} placeholder="Optional" />
      </Field>
      <div className="flex flex-wrap gap-1.5">
        {REASONS.map((r) => (
          <button key={r} type="button" aria-pressed={note === r} onClick={() => setNote(r)}
            className="rounded-pill border border-border px-3 py-1.5 text-[12px] font-semibold text-muted-foreground aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground">{r}</button>
        ))}
      </div>
    </FormSheet>
  )
}
