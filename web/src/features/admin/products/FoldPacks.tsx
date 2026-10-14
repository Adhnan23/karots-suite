import { useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminProduct, FoldRequest, FoldSource } from '@/api/types.gen'
import { INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { configQuery, errMsg } from '@/features/till/api'
import { money, num, qtyText } from '@/lib/money'

const NAMES = ['Card', 'Strip', 'Box', 'Packet', 'Bag', 'Tray']

/** "Fold <card> into packs…" on the loose twin of a card (karots-pos card + "(loose)"
 * pairs): one product left, sold by the card or by the piece. */
export function FoldPacks({ product, from, onClose }: { product: AdminProduct; from: FoldSource; onClose: () => void }) {
  const qc = useQueryClient()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const [name, setName] = useState(NAMES[0])
  const [rename, setRename] = useState(product.name.replace(/\s*\((loose|cut|single)\)\s*$/i, ''))
  const [busy, setBusy] = useState(false)
  const stock = num(from.stock)
  async function save() {
    setBusy(true)
    try {
      await api(`/admin/products/${product.id}/fold`, { json: { conversionId: from.conversionId, name, rename: rename.trim() === product.name ? '' : rename.trim() } satisfies FoldRequest })
      toast.success(`${from.name} is now a ${name} ×${qtyText(from.ratio)} of ${rename.trim() || product.name}`)
      void qc.invalidateQueries({ queryKey: ['admin'] })
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Fold ${from.name} into packs`} busy={busy} onSubmit={save} submit="Fold into one product">
      <p className="text-[14px]">One product, sold by the {name.toLowerCase()} or by the {product.unitAbbr}: no more cutting cards in the system.</p>
      <div className="flex flex-wrap gap-1.5" role="group" aria-label="Call the pack">
        {NAMES.map((n) => (
          <button key={n} type="button" aria-pressed={name === n} onClick={() => setName(n)}
            className="rounded-full border border-border px-3 py-1 text-[13px] font-semibold aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground">{n}</button>
        ))}
      </div>
      <label className="grid gap-1 text-[13px] font-semibold">Name it
        <input aria-label="Product name after folding" value={rename} onChange={(e) => setRename(e.target.value)} className={INPUT} />
      </label>
      <ul className="flex list-disc flex-col gap-1.5 pl-5 text-[13.5px] text-muted-foreground">
        <li><b className="text-foreground">{product.name}</b> gets a <b className="text-foreground">{name} ×{qtyText(from.ratio)}</b> at {money(sym, from.sellingPrice)}, and {from.name}’s barcodes scan as that {name.toLowerCase()}.</li>
        {stock > 0 && <li>The {qtyText(stock)} {from.unitAbbr} of {from.name} on the shelf are opened into {qtyText(stock * num(from.ratio))} {product.unitAbbr}, each keeping its expiry and cost.</li>}
        <li>{from.name} is turned off; its past sales stay as they were.</li>
        <li>Settings a plugin keeps on {from.name} (its alternatives group, fields) stay there — set them on {product.name} if needed.</li>
      </ul>
    </FormSheet>
  )
}
