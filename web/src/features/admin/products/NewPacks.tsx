import { Package, Plus, ScanBarcode, X } from 'lucide-react'
import { useState } from 'react'
import type { PackRequest } from '@/api/types.gen'
import { INPUT } from '@/components/admin/fields'
import { Button } from '@/components/ui/button'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { blankPack, PackFields } from './PackFields'

const BOX = 'flex flex-col gap-2 rounded-[var(--radius-sm)] border border-border p-3'

/** Packs for a product not saved yet: kept here, sent with Add product. */
export function NewPacks({ packs, onChange, unit, unitPrice, sym }: { packs: PackRequest[]; onChange: (p: PackRequest[]) => void; unit: string; unitPrice: string; sym: string }) {
  const [edit, setEdit] = useState<PackRequest | null>(null)
  const [err, setErr] = useState('')
  function add() {
    if (!edit) return
    const name = edit.name.trim().toLowerCase()
    if (packs.some((k) => k.name.trim().toLowerCase() === name)) return setErr(`This item already has a pack called ${edit.name.trim()}`)
    onChange([...packs, edit])
    setEdit(null)
    setErr('')
  }
  return (
    <section aria-label="Packs" className={BOX}>
      <div className="flex flex-wrap items-baseline justify-between gap-x-3">
        <h3 className="flex items-center gap-1.5 text-[13px] font-semibold text-muted-foreground"><Package size={15} /> Packs</h3>
        <small className="text-faint">Also sold by the card, box or packet? Add it here.</small>
      </div>
      {packs.length > 0 && (
        <ul className="flex flex-col divide-y divide-border">
          {packs.map((k) => (
            <li key={k.name} className="flex items-center gap-1 py-1.5">
              <span className="min-w-0 flex-1">
                <b className="font-semibold">{k.name} ×{qtyText(k.size)}</b>
                <span className="ml-2 tabular-nums">{money(sym, num(k.sellingPrice) > 0 ? k.sellingPrice : num(k.size) * num(unitPrice))}</span>
                {k.barcode && <small className="block text-faint tabular-nums">{k.barcode}</small>}
              </span>
              <Button type="button" variant="ghost" className="h-9 gap-1 px-2.5 text-[13px]" onClick={() => onChange(packs.filter((x) => x !== k))}><X size={14} /> Remove</Button>
            </li>
          ))}
        </ul>
      )}
      {edit ? (
        <PackFields f={edit} set={(patch) => setEdit({ ...edit, ...patch })} unit={unit} unitPrice={unitPrice} err={err} busy={false} saveLabel="Add pack"
          onCancel={() => { setEdit(null); setErr('') }} onSave={add} />
      ) : (
        <Button type="button" variant="outline" className="h-[var(--control-h)] gap-1.5 self-start px-3" onClick={() => setEdit({ ...blankPack })}>
          <Plus size={15} /> Add a pack
        </Button>
      )}
    </section>
  )
}

/** Extra barcodes for a product not saved yet. */
export function NewBarcodes({ codes, onChange }: { codes: string[]; onChange: (c: string[]) => void }) {
  const [code, setCode] = useState('')
  function add() {
    const c = code.trim()
    if (c && !codes.includes(c)) onChange([...codes, c])
    setCode('')
  }
  return (
    <section aria-label="Other barcodes" className={BOX}>
      <div className="flex flex-wrap items-baseline justify-between gap-x-3">
        <h3 className="flex items-center gap-1.5 text-[13px] font-semibold text-muted-foreground"><ScanBarcode size={15} /> Other barcodes</h3>
        <small className="text-faint">Same item, another code: every one scans to this product.</small>
      </div>
      {codes.length > 0 && (
        <ul className="flex flex-col divide-y divide-border">
          {codes.map((c) => (
            <li key={c} className="flex items-center gap-1 py-1.5">
              <b className="min-w-0 flex-1 break-all font-semibold tabular-nums">{c}</b>
              <Button type="button" variant="ghost" className="h-9 gap-1 px-2.5 text-[13px]" onClick={() => onChange(codes.filter((x) => x !== c))}><X size={14} /> Remove</Button>
            </li>
          ))}
        </ul>
      )}
      <div className="flex gap-2">
        <input aria-label="Other barcode" name="other-code" value={code} onChange={(e) => setCode(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); add() } }} placeholder="Scan or type a code" className={cn(INPUT, 'min-w-0 flex-1 tabular-nums')} />
        <Button type="button" variant="outline" className="h-[var(--control-h)] gap-1.5 px-3" disabled={!code.trim()} onClick={add}><Plus size={15} /> Add</Button>
      </div>
    </section>
  )
}
