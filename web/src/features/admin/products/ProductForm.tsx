import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { Copy, Merge, Wand2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminProduct, GeneratedBarcode, ProductRequest, ProductSaved } from '@/api/types.gen'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { Field, INPUT, Toggle } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { Button } from '@/components/ui/button'
import { CategoryPicker } from '@/features/admin/categories/CategoryPicker'
import { unitsQuery } from '@/features/admin/units/UnitsPage'
import { configQuery, errMsg } from '@/features/till/api'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { BarcodesBlock } from './BarcodesBlock'
import { MergeInto } from './MergeInto'
import { editPrice, marginText, markupOf, type Prices } from './price'
import { extraBody, type Extras, ProductExtras } from './ProductExtras'
import { productQuery, supplierOptionsQuery } from './queries'

/** id 0 = new; copyOf = duplicate that product (saved as a new one, must be renamed). */
export type Editing = { id: number; copyOf?: number }

const blank: ProductRequest = {
  name: '', nameLocal: '', barcode: '', categoryId: 0, unitId: 0, costPrice: '', sellingPrice: '', wholesalePrice: '', taxRate: '',
  reorderLevel: 0, trackSerial: false, warrantyMonths: 0, supplierId: 0, repriceStock: false, dupSourceName: '',
}

const fromProduct = (p: AdminProduct, copy: boolean): ProductRequest => ({
  ...blank, name: p.name, nameLocal: p.nameLocal, barcode: copy ? '' : p.barcode, categoryId: p.categoryId, unitId: p.unitId,
  costPrice: p.costPrice, sellingPrice: p.sellingPrice, wholesalePrice: num(p.wholesalePrice) ? p.wholesalePrice : '', taxRate: num(p.taxRate) ? p.taxRate : '',
  reorderLevel: p.reorderLevel, trackSerial: p.trackSerial, warrantyMonths: p.warrantyMonths, supplierId: p.supplierId, dupSourceName: copy ? p.name : '',
})

/** Loads what the form edits (the product, or the one being duplicated), then shows it. */
export function ProductForm({ editing, onEdit, onClose }: { editing: Editing; onEdit: (e: Editing) => void; onClose: () => void }) {
  const src = editing.id || editing.copyOf || 0
  const p = useQuery({ ...productQuery(src), enabled: src > 0 }).data
  if (src > 0 && !p) return null
  return <Form product={editing.id ? p : undefined} sourceId={editing.copyOf ?? 0} init={p ? fromProduct(p, !editing.id) : blank} onEdit={onEdit} onClose={onClose} />
}

function Form({ product, sourceId, init, onEdit, onClose }: { product?: AdminProduct; sourceId: number; init: ProductRequest; onEdit: (e: Editing) => void; onClose: () => void }) {
  const qc = useQueryClient()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const units = useQuery(unitsQuery).data ?? []
  const suppliers = useQuery(supplierOptionsQuery).data ?? []
  const [f, setF] = useState<ProductRequest>(() => ({ ...init, unitId: init.unitId || units.find((u) => u.abbreviation === 'pcs')?.id || 0 }))
  const [prices, setPrices] = useState<Prices>({ cost: init.costPrice, markup: markupOf(init.costPrice, init.sellingPrice), selling: init.sellingPrice })
  const [busy, setBusy] = useState(false)
  const [extras, setExtras] = useState<Extras>({})
  const set = (patch: Partial<ProductRequest>) => setF((x) => ({ ...x, ...patch }))
  const price = (field: keyof Prices, v: string) => setPrices((p) => editPrice(p, field, v.replace(/[^\d.]/g, '')))
  const unitId = f.unitId || units.find((u) => u.abbreviation === 'pcs')?.id || units[0]?.id || 0
  const priceChanged = !!product && num(prices.selling) !== num(product.sellingPrice)
  const onHand = product ? num(product.stockQty) : 0
  const isCopy = !!init.dupSourceName
  const svc = !!product?.isService // a service holds no stock: no supplier, reorder or serials

  const refresh = () => void qc.invalidateQueries({ queryKey: ['admin'] })
  async function save() {
    setBusy(true)
    try {
      const body: ProductRequest = { ...f, unitId, costPrice: prices.cost, sellingPrice: prices.selling, repriceStock: priceChanged && f.repriceStock, extra: extraBody(extras) }
      const r = product
        ? await api<ProductSaved>(`/admin/products/${product.id}`, { method: 'PUT', json: body })
        : await api<ProductSaved>('/admin/products', { json: body })
      toast.success(r.message)
      refresh()
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  async function act(path: string, msg: string, method = 'POST') {
    await api(path, { method })
    toast.success(msg)
    refresh()
    onClose()
  }
  async function generate() {
    try {
      set({ barcode: (await api<GeneratedBarcode>('/admin/barcode/generate')).barcode })
    } catch (e) {
      toast.error(errMsg(e))
    }
  }

  const title = product ? (product.needsReview ? `Finish ${product.name}` : `Edit ${product.name}`) : isCopy ? 'Duplicate product' : 'New product'
  return (
    <FormSheet open wide onOpenChange={(o) => !o && onClose()} title={title} busy={busy} onSubmit={save}
      submit={product?.needsReview ? 'Save & finish' : product ? 'Save' : 'Add product'}
      extra={product && (
        <div className="flex flex-wrap gap-1.5">
          {product.isActive ? (
            <ConfirmButton label="Disable" title={`Disable ${product.name}?`} confirm="Disable"
              body="It leaves the till and every picker, but keeps its sales history. You can enable it again from Show disabled."
              onConfirm={() => act(`/admin/products/${product.id}`, 'Product disabled', 'DELETE')} />
          ) : (
            <Button type="button" variant="outline" className="h-10 px-4" onClick={() => act(`/admin/products/${product.id}/activate`, 'Product enabled').catch((e) => toast.error(errMsg(e)))}>Enable</Button>
          )}
          <Button type="button" variant="ghost" className="h-10 gap-1.5 px-3" onClick={() => onEdit({ id: 0, copyOf: product.id })}><Copy size={15} /> Duplicate</Button>
          {product.isActive && (
            <MergeInto from={product} onDone={onClose} trigger={(open) => (
              <Button type="button" variant="ghost" className="h-10 gap-1.5 px-3" onClick={open}><Merge size={15} /> Merge into…</Button>
            )} />
          )}
        </div>
      )}>
      {product?.needsReview && (
        <p className="rounded-[var(--radius-sm)] bg-[color-mix(in_srgb,var(--warning)_14%,transparent)] px-3 py-2 text-[13px] text-foreground">
          Quick-added at the till{product.createdBy && ` by ${product.createdBy}`}. Set its real category, unit and cost — saving corrects the cost on the sales already made.
        </p>
      )}
      <div className="grid gap-3 md:grid-cols-2">
        <Field label="Name" hint={isCopy && f.name.trim().toLowerCase() === init.dupSourceName.toLowerCase() ? <span className="text-warning">Change the name — a copy can't keep the original's.</span> : undefined}>
          <input autoFocus required maxLength={150} name="prod-name" value={f.name} onChange={(e) => set({ name: e.target.value })} className={INPUT} />
        </Field>
        <Field label="Local name" hint="Optional — Sinhala / Tamil, printed on receipts.">
          <input name="prod-local" value={f.nameLocal} onChange={(e) => set({ nameLocal: e.target.value })} className={INPUT} />
        </Field>
        <Field label="Category"><CategoryPicker label="Category" allowCreate value={f.categoryId} onChange={(categoryId) => set({ categoryId })} /></Field>
        <Field label="Unit">
          <select aria-label="Unit" value={unitId} onChange={(e) => set({ unitId: Number(e.target.value) })} className={cn(INPUT, 'appearance-auto')}>
            {units.map((u) => <option key={u.id} value={u.id}>{u.name} ({u.abbreviation})</option>)}
          </select>
        </Field>
      </div>

      <fieldset className="grid grid-cols-2 gap-3 rounded-[var(--radius-sm)] border border-border p-3 md:grid-cols-4">
        <legend className="px-1 text-[12px] font-bold uppercase tracking-wide text-faint">Price</legend>
        <Field label="Cost"><input inputMode="decimal" aria-label="Cost" value={prices.cost} onChange={(e) => price('cost', e.target.value)} className={cn(INPUT, 'tabular-nums')} placeholder="0.00" /></Field>
        <Field label="Markup ×"><input inputMode="decimal" aria-label="Markup" value={prices.markup} onChange={(e) => price('markup', e.target.value)} className={cn(INPUT, 'tabular-nums')} placeholder="1.25" /></Field>
        <Field label="Selling" hint={marginText(prices.cost, prices.selling)}><input inputMode="decimal" required aria-label="Selling" value={prices.selling} onChange={(e) => price('selling', e.target.value)} className={cn(INPUT, 'font-bold tabular-nums')} placeholder="0.00" /></Field>
        <Field label="Wholesale" hint="Optional"><input inputMode="decimal" aria-label="Wholesale" value={f.wholesalePrice} onChange={(e) => set({ wholesalePrice: e.target.value.replace(/[^\d.]/g, '') })} className={cn(INPUT, 'tabular-nums')} placeholder="—" /></Field>
      </fieldset>
      {priceChanged && onHand > 0 && (
        <Toggle label={`Apply ${money(sym, prices.selling)} to the ${qtyText(onHand)} ${product?.unitAbbr} on hand`}
          hint="Leave off to let the stock you already have keep the price it came in at." checked={f.repriceStock} onChange={(repriceStock) => set({ repriceStock })} />
      )}

      <div className="grid gap-3 md:grid-cols-2">
        <Field label="Barcode" hint={product?.barcode ? undefined : 'Scan it, type it, or generate one to print on labels.'}>
          <div className="flex gap-1.5">
            <input name="prod-code" value={f.barcode} onChange={(e) => set({ barcode: e.target.value.trim() })} onKeyDown={(e) => e.key === 'Enter' && e.preventDefault()} className={cn(INPUT, 'min-w-0 flex-1 tabular-nums')} placeholder="Scan or type" />
            <Button type="button" variant="outline" className="h-[var(--control-h)] gap-1.5 px-3" onClick={generate} disabled={!!f.barcode}><Wand2 size={15} /> Generate</Button>
          </div>
        </Field>
        {!svc && <Field label="Preferred supplier">
          <select aria-label="Preferred supplier" value={f.supplierId} onChange={(e) => set({ supplierId: Number(e.target.value) })} className={cn(INPUT, 'appearance-auto')}>
            <option value={0}>None</option>
            {suppliers.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
          </select>
        </Field>}
        {!svc && <Field label="Reorder level" hint="Flag as running low at or below this.">
          <input inputMode="numeric" aria-label="Reorder level" value={f.reorderLevel || ''} onChange={(e) => set({ reorderLevel: Number(e.target.value.replace(/\D/g, '')) || 0 })} className={INPUT} placeholder="0" />
        </Field>}
        <Field label="Tax %" hint="Leave empty for none.">
          <input inputMode="decimal" aria-label="Tax" value={f.taxRate} onChange={(e) => set({ taxRate: e.target.value.replace(/[^\d.]/g, '') })} className={INPUT} placeholder="0" />
        </Field>
      </div>
      {product && <BarcodesBlock productId={product.id} onMain={(barcode) => set({ barcode })} />}
      {!svc && <div className="grid gap-3 md:grid-cols-[1fr_200px] md:items-end">
        <Toggle label="Track serial numbers" hint="Each unit sold records its serial, for warranty claims." checked={f.trackSerial} onChange={(trackSerial) => set({ trackSerial })} />
        <Field label="Warranty (months)">
          <input inputMode="numeric" aria-label="Warranty months" value={f.warrantyMonths || ''} onChange={(e) => set({ warrantyMonths: Math.min(600, Number(e.target.value.replace(/\D/g, '')) || 0) })} className={INPUT} placeholder="None" />
        </Field>
      </div>}
      <ProductExtras productId={product?.id ?? 0} sourceId={sourceId} categoryId={f.categoryId} where="form" extras={extras} setExtras={setExtras} />
    </FormSheet>
  )
}
