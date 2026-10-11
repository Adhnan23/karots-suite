import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { GeneratedBarcode, TillProduct } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { slots } from '@/plugins/slots'
import { errMsg, unitsQuery } from '../api'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'
import { FIELD } from '../ui'

/** Sell something not in the catalogue yet (it's created for admin review). */
export function QuickItem({ barcode: initialBarcode, name: initialName }: { barcode: string; name: string }) {
  const ui = useTillUi()
  const { dispatch } = useTill()
  const qc = useQueryClient()
  const units = useQuery(unitsQuery).data ?? []
  const [name, setName] = useState(initialName)
  const [price, setPrice] = useState('')
  const [qty, setQty] = useState('1')
  const [barcode, setBarcode] = useState(initialBarcode)
  const [unitId, setUnitId] = useState<number>(0)
  const unit = units.find((u) => u.id === unitId) ?? units.find((u) => u.abbreviation === 'pcs') ?? units[0]

  async function generate() {
    try {
      setBarcode((await api<GeneratedBarcode>('/till/barcode/generate')).barcode)
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  async function submit() {
    if (!name.trim()) return void toast.error('Enter an item name')
    if (price.trim() === '' || !(num(price) >= 0)) return void toast.error('Enter a price')
    let q = num(qty)
    if (!unit?.allowDecimal) q = Math.round(q)
    if (!(q > 0)) q = 1
    try {
      const p = await api<TillProduct>('/till/quick-item', {
        json: { name: name.trim(), price: String(num(price)), qty: String(q), barcode: barcode.trim(), unitId: unit?.id ?? 0 },
      })
      dispatch({ type: 'quickSet', product: p, qty: q })
      toast.success('Item added')
      ui.close()
      void qc.invalidateQueries({ queryKey: ['till', 'search'] })
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  return (
    <Dialog open onOpenChange={(o) => !o && ui.close()}>
      <DialogContent className="sm:max-w-[460px]">
        <DialogHeader>
          <DialogTitle>Quick item</DialogTitle>
          <DialogDescription>Sell an item that isn’t in the catalogue yet. An admin can finish its details later.</DialogDescription>
        </DialogHeader>
        <form className="grid grid-cols-2 gap-3" onSubmit={(e) => { e.preventDefault(); void submit() }}>
          <input autoFocus aria-label="Item name" placeholder="Item name" autoComplete="off" value={name} onChange={(e) => setName(e.target.value)} className={cn(FIELD, 'col-span-2')} />
          <input aria-label="Price" placeholder="Price" inputMode="decimal" autoComplete="off" value={price} onChange={(e) => setPrice(e.target.value.replace(/[^\d.]/g, ''))} className={FIELD} />
          <div className="flex gap-2">
            <input aria-label="Quantity" inputMode="decimal" autoComplete="off" value={qty} onChange={(e) => setQty(e.target.value.replace(/[^\d.]/g, ''))} className={cn(FIELD, 'w-20')} />
            <select aria-label="Unit" value={unit?.id ?? 0} onChange={(e) => setUnitId(Number(e.target.value))} className={cn(FIELD, 'min-w-0 flex-1 appearance-auto')}>
              {units.map((u) => <option key={u.id} value={u.id}>{u.name}</option>)}
            </select>
          </div>
          <div className="col-span-2 flex gap-2">
            <input aria-label="Barcode" placeholder="Barcode (optional)" autoComplete="off" value={barcode} onChange={(e) => setBarcode(e.target.value)} className={cn(FIELD, 'min-w-0 flex-1')} />
            <Button type="button" variant="secondary" className="h-[var(--control-h)]" onClick={generate}>Generate</Button>
          </div>
          {slots.tillActions.filter((a) => a.noMatch).map((a) => (
            <button key={a.key} type="button" className="col-span-2 justify-self-start text-sm font-medium text-brand hover:underline"
              onClick={() => ui.open({ kind: 'action', key: a.key, text: name.trim() })}>
              {a.noMatch}
            </button>
          ))}
          <DialogFooter className="col-span-2">
            <Button type="button" variant="ghost" onClick={ui.close}>Cancel</Button>
            <Button type="submit">Add to cart</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
