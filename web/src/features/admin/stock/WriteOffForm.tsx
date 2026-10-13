import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminProductRow, ConsumeRequest, ConsumeResult } from '@/api/types.gen'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { ProductPicker } from '@/features/admin/products/ProductPicker'
import { Seg } from '@/features/till/bits'
import { configQuery, errMsg } from '@/features/till/api'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { lotsQuery } from './queries'

type Reason = 'damage' | 'own_use' | 'staff'
const REASONS: [Reason, string][] = [['damage', 'Damaged / expired'], ['own_use', 'Shop use'], ['staff', 'Staff']]
const WHY: Record<Reason, string> = {
  damage: 'Broken or past its date — written off as a loss.',
  own_use: 'The shop used it — counter supplies, cleaning.',
  staff: 'Staff took it — food, drinks.',
}

/** Take stock out without a sale (karots-pos Record Damage): damage, shop use or
 * staff, from the lot it really came out of when there are several. */
export function WriteOffForm({ onClose, onDamaged }: {
  onClose: () => void
  /** A damage write-off: ask whether the supplier will make it good (useSupplierClaim). */
  onDamaged: (lossId: number, product: string, qty: string, value: number) => void
}) {
  const sym = useSuspenseQuery(configQuery).data.symbol
  const qc = useQueryClient()
  const [p, setP] = useState<AdminProductRow | null>(null)
  const lots = useQuery({ ...lotsQuery(p?.id ?? 0), enabled: !!p }).data?.lots ?? []
  const [lot, setLot] = useState(0)
  const [qty, setQty] = useState('')
  const [reason, setReason] = useState<Reason>('damage')
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const chosen = lots.find((l) => l.id === (lot || lots[0]?.id))
  async function save() {
    if (!p) return
    setBusy(true)
    try {
      const body: ConsumeRequest = { productId: p.id, quantity: qty, reason, note, batchId: lots.length > 1 ? chosen?.id ?? 0 : 0 }
      const r = await api<ConsumeResult>('/admin/stock/damage', { json: body })
      if (r.lossId) onDamaged(r.lossId, p.name, qty, num(qty) * num(chosen?.costPrice ?? p.costPrice))
      else toast.success(`${p.name}: −${qtyText(qty)} ${p.unitAbbr} (${REASONS.find(([r]) => r === reason)?.[1].toLowerCase()})`)
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
    <FormSheet open onOpenChange={(o) => !o && onClose()} title="Record a write-off" submit="Write off" busy={busy || !p || !(num(qty) > 0)} onSubmit={save}>
      <Field label="Product">
        <ProductPicker title="What’s being written off" onPick={(x) => { setP(x); setLot(0) }} trigger={(open) => (
          <button type="button" aria-label="Product" onClick={open} className={cn(INPUT, 'truncate text-left', !p && 'text-faint')}>{p ? `${p.name} · ${qtyText(p.stockQty)} ${p.unitAbbr} on hand` : 'Pick a product'}</button>
        )} />
      </Field>
      <div className="flex flex-col gap-1.5">
        <Seg label="Reason" value={reason} onChange={setReason} options={REASONS} />
        <small className="text-faint">{WHY[reason]}</small>
      </div>
      {lots.length > 1 && (
        <fieldset className="flex flex-col gap-1.5">
          <legend className="mb-1.5 text-[13px] font-semibold text-muted-foreground">Which lot?</legend>
          {lots.map((l, i) => (
            <label key={l.id} className="flex cursor-pointer items-center gap-3 rounded-[var(--radius-sm)] border border-border px-3 py-2 has-checked:border-selected has-checked:bg-muted">
              <input type="radio" name="wo-lot" checked={chosen?.id === l.id} onChange={() => setLot(l.id)} />
              <span className="text-sm"><b>{qtyText(l.qtyRemaining)} {p?.unitAbbr}</b> at {money(sym, l.sellingPrice || p?.sellingPrice)}
                <small className="block text-faint">{i === 0 ? 'oldest · ' : ''}{l.expiryDate ? `expires ${l.expiryDate} · ` : ''}cost {money(sym, l.costPrice)}</small></span>
            </label>
          ))}
        </fieldset>
      )}
      <Field label="Quantity" hint={p && num(qty) > 0 ? `worth ${money(sym, num(qty) * num(chosen?.costPrice ?? p.costPrice))} at cost` : undefined}>
        <input aria-label="Quantity" inputMode="decimal" value={qty} onChange={(e) => setQty(e.target.value.replace(/[^\d.]/g, ''))} className={cn(INPUT, 'tabular-nums')} placeholder="0" />
      </Field>
      <Field label="Note"><input aria-label="Note" value={note} onChange={(e) => setNote(e.target.value)} className={INPUT} placeholder="What happened (optional)" /></Field>
    </FormSheet>
  )
}
