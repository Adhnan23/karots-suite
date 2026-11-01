import { ScanBarcode } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { api } from '@/api/client'
import type { GeneratedBarcode, TillProduct, Unit } from '@/api/types.gen'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { FIELD } from '@/features/till/ui'
import { num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { createQuickItem, liveCall, PC_DOWN } from '../quickItem'
import { Scanner } from './Scanner'

/** Sell something not in the catalogue yet — the PC's Quick item (made on the PC
 * for admin review), so live only: offline it says so in place. */
export function QuickItemSheet({ name: firstName, barcode: firstCode, live, onAdded, onClose }: {
  name: string
  barcode: string
  live: boolean
  onAdded: (p: TillProduct, qty: number) => void
  onClose: () => void
}) {
  const [name, setName] = useState(firstName)
  const [price, setPrice] = useState('')
  const [qty, setQty] = useState('1')
  const [barcode, setBarcode] = useState(firstCode)
  const [units, setUnits] = useState<Unit[]>([])
  const [unitId, setUnitId] = useState(0)
  const [scan, setScan] = useState(false)
  const [error, setError] = useState(live ? '' : PC_DOWN)
  const [busy, setBusy] = useState(false)
  useEffect(() => {
    if (live) api<Unit[]>('/till/units').then(setUnits, () => {})
  }, [live])
  const scanned = useCallback((c: string) => { setScan(false); setBarcode(c) }, [])
  const unit = units.find((u) => u.id === unitId) ?? units.find((u) => u.abbreviation === 'pcs') ?? units[0]

  async function generate() {
    const r = await liveCall<GeneratedBarcode>('/till/barcode/generate', undefined)
    if ('ok' in r) setBarcode(r.ok.barcode)
    else setError(r.error)
  }
  async function submit() {
    if (!name.trim()) return setError('Enter an item name')
    if (price.trim() === '' || !(num(price) >= 0)) return setError('Enter a price')
    let q = num(qty)
    if (!unit?.allowDecimal) q = Math.round(q)
    if (!(q > 0)) q = 1
    setBusy(true)
    const r = await createQuickItem({ name: name.trim(), price: String(num(price)), qty: String(q), barcode: barcode.trim(), unitId: unit?.id ?? 0 })
    setBusy(false)
    if ('error' in r) return setError(r.error)
    onAdded(r.product, q)
    onClose()
  }

  return (
    <Sheet open onOpenChange={(o) => !o && onClose()}>
      <SheetContent side="bottom" className="max-h-[92dvh] gap-3 overflow-y-auto p-4">
        <SheetHeader className="p-0">
          <SheetTitle>Quick item</SheetTitle>
          <SheetDescription>Sell an item that isn’t in the catalogue yet. An admin can finish its details later.</SheetDescription>
        </SheetHeader>
        <form className="grid grid-cols-2 gap-2.5" onSubmit={(e) => { e.preventDefault(); void submit() }}>
          <input aria-label="Item name" placeholder="Item name" autoComplete="off" value={name} onChange={(e) => { setName(e.target.value); setError(live ? '' : PC_DOWN) }} className={cn(FIELD, 'col-span-2')} />
          <input aria-label="Price" placeholder="Price" inputMode="decimal" autoComplete="off" value={price} onChange={(e) => setPrice(e.target.value.replace(/[^\d.]/g, ''))} className={FIELD} />
          <div className="flex gap-2">
            <input aria-label="Quantity" inputMode="decimal" autoComplete="off" value={qty} onChange={(e) => setQty(e.target.value.replace(/[^\d.]/g, ''))} className={cn(FIELD, 'w-16')} />
            {units.length > 0 && (
              <select aria-label="Unit" value={unit?.id ?? 0} onChange={(e) => setUnitId(Number(e.target.value))} className={cn(FIELD, 'min-w-0 flex-1 appearance-auto')}>
                {units.map((u) => <option key={u.id} value={u.id}>{u.name}</option>)}
              </select>
            )}
          </div>
          <div className="col-span-2 flex gap-2">
            <input aria-label="Barcode" placeholder="Barcode (optional)" autoComplete="off" value={barcode} onChange={(e) => setBarcode(e.target.value)} className={cn(FIELD, 'min-w-0 flex-1')} />
            <button type="button" aria-label="Scan the barcode" onClick={() => setScan(true)} className="grid w-11 place-items-center rounded-[var(--radius-sm)] bg-muted"><ScanBarcode size={18} /></button>
            <button type="button" onClick={() => void generate()} disabled={!live} className="rounded-[var(--radius-sm)] bg-muted px-3 text-sm font-semibold disabled:opacity-50">Generate</button>
          </div>
          {error && <p role="alert" className="col-span-2 text-sm font-semibold text-destructive">{error}</p>}
          <button type="submit" disabled={busy || !live} className="col-span-2 rounded-[var(--radius)] bg-brand p-3.5 text-base font-extrabold text-brand-foreground disabled:opacity-60">
            {busy ? 'Adding…' : 'Add to cart'}
          </button>
        </form>
        {scan && <Scanner onCode={scanned} onClose={() => setScan(false)} />}
      </SheetContent>
    </Sheet>
  )
}
