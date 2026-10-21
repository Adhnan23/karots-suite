import { Minus, Plus, X } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { FormSheet } from '@/components/admin/FormSheet'
import { Field, INPUT } from '@/components/admin/fields'
import { Button } from '@/components/ui/button'
import { ProductPicker } from '@/features/admin/products/ProductPicker'
import { errMsg } from '@/features/till/api'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { saveCombo } from './api'
import { setPrice } from './pricing'
import type { Combo, DiscountType, Member } from './types'

const TYPES: [DiscountType, string][] = [['price', 'Combo price'], ['percent', '% off'], ['fixed', 'Rs off']]
const SEG = 'flex-1 rounded-[var(--radius-sm)] px-3 py-2 text-[13px] font-semibold text-muted-foreground aria-pressed:bg-selected aria-pressed:text-selected-foreground'

type Draft = { name: string; discountType: DiscountType; discountValue: string; startsOn: string; endsOn: string; members: Member[] }

const draftOf = (c: Combo | null): Draft => c
  ? { name: c.name, discountType: c.discountType, discountValue: String(Number(c.discountValue)), startsOn: c.startsOn, endsOn: c.endsOn, members: c.members }
  : { name: '', discountType: 'price', discountValue: '', startsOn: '', endsOn: '', members: [] }

/** New / edit combo: name, how it's priced, the items (×qty) and optional dates. */
export function ComboSheet({ combo, open, onClose, sym }: { combo: Combo | null; open: boolean; onClose: () => void; sym: string }) {
  const [d, setD] = useState<Draft>(() => draftOf(combo))
  const [busy, setBusy] = useState(false)
  const set = (patch: Partial<Draft>) => setD((x) => ({ ...x, ...patch }))
  const setQty = (id: number, qty: number) =>
    set({ members: d.members.flatMap((m) => (m.productId !== id ? [m] : qty > 0 ? [{ ...m, qty: String(qty) }] : [])) })
  const units = d.members.reduce((s, m) => s + Number(m.qty), 0)
  const { full, price } = setPrice({ discountType: d.discountType, discountValue: d.discountValue || '0', members: d.members }, 'retail')
  const submit = async () => {
    setBusy(true)
    try {
      await saveCombo(combo?.id ?? 0, { name: d.name, discountType: d.discountType, discountValue: d.discountValue || '0',
        startsOn: d.startsOn, endsOn: d.endsOn, items: d.members.map((m) => ({ productId: m.productId, qty: m.qty })) })
      toast.success(combo ? 'Combo saved' : 'Combo added')
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open={open} onOpenChange={(o) => !o && onClose()} title={combo ? 'Edit combo' : 'New combo'} submit="Save combo" busy={busy} onSubmit={submit}>
      <Field label="Name">
        <input required autoFocus value={d.name} onChange={(e) => set({ name: e.target.value })} placeholder="Milk + Cookies" className={INPUT} />
      </Field>
      <div className="flex flex-col gap-1.5 text-[13px] font-semibold text-muted-foreground">
        Items in one set
        <ul className="flex flex-col gap-1.5">
          {d.members.map((m) => (
            <li key={m.productId} className="flex items-center gap-2 rounded-[var(--radius-sm)] border border-border bg-muted px-3 py-1.5 text-[14px] text-foreground">
              <span className="min-w-0 flex-1 truncate font-semibold">{m.name}</span>
              <span className="text-[12.5px] font-normal text-faint tabular-nums">{money(sym, m.sellingPrice)}</span>
              <span className="flex items-center rounded-pill border border-border bg-card">
                <button type="button" aria-label={`Fewer ${m.name}`} onClick={() => setQty(m.productId, Number(m.qty) - 1)} className="grid size-7 place-items-center"><Minus size={13} /></button>
                <span className="w-6 text-center font-bold tabular-nums">{Number(m.qty)}</span>
                <button type="button" aria-label={`More ${m.name}`} onClick={() => setQty(m.productId, Number(m.qty) + 1)} className="grid size-7 place-items-center"><Plus size={13} /></button>
              </span>
              <button type="button" aria-label={`Take out ${m.name}`} onClick={() => setQty(m.productId, 0)} className="text-faint hover:text-danger"><X size={15} /></button>
            </li>
          ))}
        </ul>
        <ProductPicker title="Add an item" exclude={new Set(d.members.map((m) => m.productId))}
          onPick={(p) => set({ members: [...d.members, { productId: p.id, name: p.name, qty: '1', sellingPrice: p.sellingPrice, wholesalePrice: p.wholesalePrice,
            active: p.isActive, isService: false, stock: p.stockQty }] })}
          trigger={(o) => <Button type="button" variant="secondary" className="h-10 self-start" onClick={o}><Plus size={15} /> Add an item</Button>} />
        {units < 2 && <small className="font-normal text-faint">At least 2 units: two products, or one product ×2 or more (buy 3 get 1 free = the item ×4).</small>}
      </div>
      <div className="flex flex-col gap-1.5 text-[13px] font-semibold text-muted-foreground">
        Deal
        <div role="group" aria-label="How the combo is priced" className="flex gap-1 rounded-[var(--radius-sm)] border border-border bg-muted p-1">
          {TYPES.map(([t, label]) => <button key={t} type="button" aria-pressed={d.discountType === t} className={SEG} onClick={() => set({ discountType: t })}>{label}</button>)}
        </div>
        <input aria-label={TYPES.find(([t]) => t === d.discountType)![1]} inputMode="decimal" autoComplete="off" value={d.discountValue}
          onChange={(e) => set({ discountValue: e.target.value.replace(/[^\d.]/g, '') })}
          placeholder={d.discountType === 'price' ? 'Price of one set' : d.discountType === 'percent' ? 'Percent off a set' : 'Amount off a set'} className={INPUT} />
        {d.members.length > 0 && (
          <p className={cn('rounded-[var(--radius-sm)] bg-brand-soft px-3 py-2 text-[14px] text-foreground tabular-nums')}>
            One set: <s className="text-faint">{money(sym, full / 100)}</s> → <b className="font-display">{money(sym, price / 100)}</b>
            {full > price && <span className="text-success"> · saves {money(sym, (full - price) / 100)}</span>}
          </p>
        )}
      </div>
      <div className="grid grid-cols-2 gap-3">
        <Field label="Starts" hint="Blank = now"><input type="date" value={d.startsOn} onChange={(e) => set({ startsOn: e.target.value })} className={INPUT} /></Field>
        <Field label="Ends" hint="Blank = no end"><input type="date" value={d.endsOn} min={d.startsOn || undefined} onChange={(e) => set({ endsOn: e.target.value })} className={INPUT} /></Field>
      </div>
    </FormSheet>
  )
}
