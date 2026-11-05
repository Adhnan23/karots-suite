import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Package, Pencil, Plus, X } from 'lucide-react'
import { useState } from 'react'
import { api } from '@/api/client'
import type { AdminPack, PackRequest } from '@/api/types.gen'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'
import { packGross } from '@/features/till/cart/packs'
import { money, num, qtyText } from '@/lib/money'
import { blankPack, PackFields } from './PackFields'
import { packsQuery } from './queries'

const blank = blankPack

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
        <PackFields f={edit.f} set={set} unit={unit} unitPrice={unitPrice} err={err} busy={busy} saveLabel={edit.id ? 'Save pack' : 'Add pack'}
          onCancel={() => setEdit(null)} onSave={() => void save()} />
      ) : (
        <Button type="button" variant="outline" className="h-[var(--control-h)] gap-1.5 self-start px-3" onClick={() => { setErr(''); setEdit({ id: 0, f: blank }) }}>
          <Plus size={15} /> Add a pack
        </Button>
      )}
    </section>
  )
}
