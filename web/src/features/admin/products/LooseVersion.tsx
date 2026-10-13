import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { Wand2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminProduct, GeneratedBarcode, LooseVersionRequest, LooseVersionResult } from '@/api/types.gen'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { Button } from '@/components/ui/button'
import { convQuery } from '@/features/admin/conversions/ConversionsPage'
import { unitsQuery } from '@/features/admin/units/UnitsPage'
import { configQuery, errMsg } from '@/features/till/api'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { marginText } from './price'
import { extraBody, type Extras, ProductExtras } from './ProductExtras'

const clean = (v: string) => v.replace(/[^\d.]/g, '')

/** "Make a loose version…": a packed product (a 25 kg bag) gets its loose twin sold by
 * another unit (kg) — the product, the conversion and, if asked, the first bags opened,
 * in one save. Category, supplier, tax, reorder, warranty and local name follow the bag. */
export function LooseVersion({ product, onClose, onOpen }: { product: AdminProduct; onClose: () => void; onOpen: (id: number) => void }) {
  const qc = useQueryClient()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const units = (useQuery(unitsQuery).data ?? []).filter((u) => u.id !== product.unitId)
  const already = (useQuery(convQuery('', false)).data ?? []).filter((c) => c.fromProductId === product.id)
  const [f, setF] = useState<LooseVersionRequest>({ name: `${product.name} (loose)`, unitId: 0, ratio: '', sellingPrice: '', barcode: '', convertNow: '' })
  const [extras, setExtras] = useState<Extras>({})
  const [busy, setBusy] = useState(false)
  const set = (patch: Partial<LooseVersionRequest>) => setF((x) => ({ ...x, ...patch }))
  const unitId = f.unitId || units.find((u) => u.abbreviation === 'kg')?.id || units[0]?.id || 0
  const unit = units.find((u) => u.id === unitId)?.abbreviation ?? ''
  const ratio = num(f.ratio)
  const costPer = ratio > 0 ? (num(product.costPrice) / ratio).toFixed(2) : ''
  const now = num(f.convertNow)
  const onHand = num(product.stockQty)
  const tooMany = now > onHand

  async function save() {
    setBusy(true)
    try {
      const r = await api<LooseVersionResult>(`/admin/products/${product.id}/loose`, { json: { ...f, unitId, extra: extraBody(extras) } })
      toast.success(now > 0 ? `${f.name} made — ${qtyText(now * ratio)} ${unit} in stock` : `${f.name} made`)
      void qc.invalidateQueries({ queryKey: ['admin'] })
      void qc.invalidateQueries({ queryKey: ['till'] })
      onOpen(r.productId)
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  async function generate() {
    try {
      set({ barcode: (await api<GeneratedBarcode>('/admin/barcode/generate')).barcode })
    } catch (e) {
      toast.error(errMsg(e))
    }
  }

  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Sell ${product.name} loose`} submit="Make loose version"
      busy={busy || !f.name.trim() || !(ratio > 0) || !unitId || tooMany} onSubmit={save}>
      {already.length > 0 && (
        <p className="rounded-[var(--radius-sm)] bg-muted px-3 py-2 text-[13px]">
          Already sold loose as{' '}
          {already.map((c, i) => (
            <span key={c.id}>{i > 0 && ', '}
              <button type="button" className="font-semibold text-brand underline-offset-2 hover:underline" onClick={() => onOpen(c.toProductId)}>{c.toName}</button>
              {' '}<span className="text-faint">({qtyText(c.ratio)} {c.toUnit})</span>
            </span>
          ))}. Another one is fine — say, kg and 500 g.
        </p>
      )}
      <Field label="Name">
        <input autoFocus required maxLength={150} name="loose-name" value={f.name} onChange={(e) => set({ name: e.target.value })} className={INPUT} />
      </Field>
      <div className="grid grid-cols-2 gap-3">
        <Field label="Sold by">
          <select aria-label="Loose unit" value={unitId} onChange={(e) => set({ unitId: Number(e.target.value) })} className={cn(INPUT, 'appearance-auto')}>
            {units.map((u) => <option key={u.id} value={u.id}>{u.name} ({u.abbreviation})</option>)}
          </select>
        </Field>
        <Field label={`1 ${product.unitAbbr} makes`} hint={costPer ? `Costs about ${money(sym, costPer)} a ${unit}` : `How many ${unit} in one ${product.unitAbbr}`}>
          <div className="flex items-center gap-2">
            <input aria-label="Ratio" inputMode="decimal" required value={f.ratio} onChange={(e) => set({ ratio: clean(e.target.value) })} className={cn(INPUT, 'min-w-0 flex-1 tabular-nums')} placeholder="25" />
            <span className="text-[13px] font-semibold text-muted-foreground">{unit}</span>
          </div>
        </Field>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <Field label={`Selling a ${unit || 'unit'}`} hint={marginText(costPer, f.sellingPrice)}>
          <input aria-label="Loose selling price" inputMode="decimal" value={f.sellingPrice} onChange={(e) => set({ sellingPrice: clean(e.target.value) })} className={cn(INPUT, 'font-bold tabular-nums')} placeholder="0.00" />
        </Field>
        <Field label={`Open now (${product.unitAbbr})`} hint={tooMany ? <span className="text-danger">Only {qtyText(onHand)} on hand</span> : now > 0 && ratio > 0 ? `→ ${qtyText(now * ratio)} ${unit} in stock` : `${qtyText(onHand)} on hand · optional`}>
          <input aria-label="Convert now" inputMode="decimal" value={f.convertNow} onChange={(e) => set({ convertNow: clean(e.target.value) })} className={cn(INPUT, 'tabular-nums', tooMany && 'border-danger')} placeholder="0" />
        </Field>
      </div>
      <Field label="Barcode" hint="Optional — loose goods are usually picked on screen.">
        <div className="flex gap-1.5">
          <input name="loose-code" value={f.barcode} onChange={(e) => set({ barcode: e.target.value.trim() })} onKeyDown={(e) => e.key === 'Enter' && e.preventDefault()} className={cn(INPUT, 'min-w-0 flex-1 tabular-nums')} placeholder="Scan or type" />
          <Button type="button" variant="outline" className="h-[var(--control-h)] gap-1.5 px-3" onClick={generate} disabled={!!f.barcode}><Wand2 size={15} /> Generate</Button>
        </div>
      </Field>
      <ProductExtras productId={0} sourceId={product.id} kind="loose" categoryId={product.categoryId} where="form" extras={extras} setExtras={setExtras} />
    </FormSheet>
  )
}
