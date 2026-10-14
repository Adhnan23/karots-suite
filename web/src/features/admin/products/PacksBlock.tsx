import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Package, Pencil, Plus, X } from 'lucide-react'
import { useState } from 'react'
import { api } from '@/api/client'
import type { AdminPack, PackRequest } from '@/api/types.gen'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { INPUT } from '@/components/admin/fields'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'
import { packGross } from '@/features/till/cart/packs'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { packsQuery } from './queries'

const NAMES = ['Card', 'Box', 'Packet', 'Strip', 'Tray', 'Bottle']
const blank: PackRequest = { name: 'Card', size: '', sellingPrice: '', wholesalePrice: '', barcode: '' }

/** Ways to sell (and buy) the item by more than one unit: Card ×12 with its own price and
 * the code printed on the card. Stock stays in units; full packs ring at the pack price. */
export function PacksBlock({ productId, unit, unitPrice, stockQty, sym }: { productId: number; unit: string; unitPrice: string; stockQty: string; sym: string }) {
  const qc = useQueryClient()
  const packs = useQuery(packsQuery(productId)).data ?? []
  const [edit, setEdit] = useState<{ id: number; f: PackRequest } | null>(null)
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const base = `/admin/products/${productId}/packs`
  const refresh = () => qc.invalidateQueries({ queryKey: ['admin'] })
  const split = packs.length ? packGross(num(stockQty), 0, packs.map((k) => ({ id: k.id, name: k.name, size: num(k.size), price: 0 }))).label : ''

  async function save() {
    if (!edit || busy) return
    setBusy(true)
    try {
      await api(edit.id ? `${base}/${edit.id}` : base, { method: edit.id ? 'PUT' : 'POST', json: edit.f })
      setEdit(null)
      setErr('')
      await refresh()
    } catch (e) {
      setErr(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  const set = (patch: Partial<PackRequest>) => setEdit((x) => x && { ...x, f: { ...x.f, ...patch } })
  const follows = (k: AdminPack) => num(k.size) * num(unitPrice)

  return (
    <section aria-label="Packs" className="flex flex-col gap-2 rounded-[var(--radius-sm)] border border-border p-3">
      <div className="flex flex-wrap items-baseline justify-between gap-x-3">
        <h3 className="flex items-center gap-1.5 text-[13px] font-semibold text-muted-foreground"><Package size={15} /> Packs</h3>
        <small className="text-faint">
          {packs.length ? `Stock ${qtyText(stockQty)} ${unit}${split ? ` = ${split}` : ''}` : `Also sold by the card, box or packet? Stock stays in ${unit}.`}
        </small>
      </div>
      {packs.length > 0 && (
        <ul className="flex flex-col divide-y divide-border">
          {packs.map((k) => (
            <li key={k.id} className="flex flex-wrap items-center gap-x-1 gap-y-0.5 py-1.5">
              <span className="min-w-0 basis-full sm:flex-1 sm:basis-auto">
                <b className="font-semibold">{k.name} ×{qtyText(k.size)}</b>
                <span className="ml-2 tabular-nums">{num(k.sellingPrice) > 0 ? money(sym, k.sellingPrice) : `${money(sym, follows(k))} (follows the unit price)`}</span>
                {num(k.wholesalePrice) > 0 && <span className="ml-2 text-muted-foreground tabular-nums">wholesale {money(sym, k.wholesalePrice)}</span>}
                {k.setAtTill && <span className="ml-2 rounded-md bg-[color-mix(in_srgb,var(--warning)_16%,transparent)] px-1.5 text-[11.5px] font-semibold">set at the till — check it</span>}
                {k.barcodes.length > 0 && <small className="block text-faint tabular-nums">{k.barcodes.join(' · ')}</small>}
              </span>
              <Button type="button" variant="ghost" className="-ml-2.5 h-9 gap-1 px-2.5 text-[13px] sm:ml-0"
                onClick={() => { setErr(''); setEdit({ id: k.id, f: { name: k.name, size: qtyText(k.size), sellingPrice: num(k.sellingPrice) ? k.sellingPrice : '', wholesalePrice: num(k.wholesalePrice) ? k.wholesalePrice : '', barcode: '' } }) }}>
                <Pencil size={14} /> Edit
              </Button>
              <ConfirmButton plain className="h-9 px-2.5 text-[13px]" label={<><X size={14} /> Remove</>} title={`Remove ${k.name} ×${qtyText(k.size)}?`}
                body="The till sells it by the unit only, and the pack's barcodes stop scanning. Past sales keep what they say."
                confirm="Remove" onConfirm={() => api(`${base}/${k.id}`, { method: 'DELETE' }).then(refresh)} />
            </li>
          ))}
        </ul>
      )}
      {edit ? (
        <div className="grid gap-2 rounded-[var(--radius-sm)] bg-muted p-2.5" onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); void save() } }}>
          <div className="flex flex-wrap gap-1.5">
            {NAMES.map((n) => (
              <button key={n} type="button" aria-pressed={edit.f.name === n} onClick={() => set({ name: n })}
                className="rounded-full border border-border bg-card px-3 py-1 text-[13px] font-semibold aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground">{n}</button>
            ))}
          </div>
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
            <label className="grid gap-1 text-[12.5px] font-semibold">Name
              <input aria-label="Pack name" value={edit.f.name} onChange={(e) => set({ name: e.target.value })} className={INPUT} />
            </label>
            <label className="grid gap-1 text-[12.5px] font-semibold">{unit} in one
              <input aria-label="Units in one pack" inputMode="numeric" value={edit.f.size} onChange={(e) => set({ size: e.target.value.replace(/[^\d.]/g, '') })} className={cn(INPUT, 'tabular-nums')} placeholder="12" />
            </label>
            <label className="grid gap-1 text-[12.5px] font-semibold">Price
              <input aria-label="Pack price" inputMode="decimal" value={edit.f.sellingPrice} onChange={(e) => set({ sellingPrice: e.target.value.replace(/[^\d.]/g, '') })}
                className={cn(INPUT, 'tabular-nums')} placeholder={num(edit.f.size) > 1 ? (num(edit.f.size) * num(unitPrice)).toFixed(2) : '0.00'} />
            </label>
            <label className="grid gap-1 text-[12.5px] font-semibold">Wholesale
              <input aria-label="Pack wholesale price" inputMode="decimal" value={edit.f.wholesalePrice} onChange={(e) => set({ wholesalePrice: e.target.value.replace(/[^\d.]/g, '') })} className={cn(INPUT, 'tabular-nums')} placeholder="—" />
            </label>
          </div>
          <input aria-label="Pack barcode" name="pack-code" value={edit.f.barcode} onChange={(e) => set({ barcode: e.target.value })}
            placeholder="Scan the code on the pack (optional)" className={cn(INPUT, 'tabular-nums')} />
          <small className="text-faint">A blank price follows {unit} price × size. Full packs always ring at the cheaper of the two.</small>
          {err && <small role="alert" className="text-danger">{err}</small>}
          <div className="flex justify-end gap-1.5">
            <Button type="button" variant="ghost" className="h-9" onClick={() => setEdit(null)}>Cancel</Button>
            <Button type="button" className="h-9 font-bold" disabled={busy || !(num(edit.f.size) > 1) || !edit.f.name.trim()} onClick={() => void save()}>
              {edit.id ? 'Save pack' : 'Add pack'}
            </Button>
          </div>
        </div>
      ) : (
        <Button type="button" variant="outline" className="h-[var(--control-h)] gap-1.5 self-start px-3" onClick={() => { setErr(''); setEdit({ id: 0, f: blank }) }}>
          <Plus size={15} /> Add a pack
        </Button>
      )}
    </section>
  )
}
