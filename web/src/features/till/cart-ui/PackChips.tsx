import { Package, Plus } from 'lucide-react'
import { useState } from 'react'
import { api } from '@/api/client'
import type { TillPack } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { errMsg } from '../api'
import { packed, packGross, packPlural } from '../cart/packs'
import type { CartLine } from '../cart/types'
import { useTill } from '../TillProvider'
import { FIELD } from '../ui'

const CHIP = 'flex h-7 items-center gap-1 rounded-full px-2.5 text-[12.5px] font-semibold'
const NAMES = ['Card', 'Box', 'Packet', 'Strip', 'Tray', 'Bottle']

/** Under a cart line of something sold by the pack: the split as handed over
 * ("= 1 Card + 2") and + one pack. A whole-unit line with no pack gets a quiet
 * "Set pack" to set one up right here (the cashier reads the size off the box). */
export function PackChips({ line, sym, onQty }: { line: CartLine; sym: string; onQty: (q: number) => void }) {
  const [asking, setAsking] = useState(false)
  if (line.isService || line.group || line.fixedQty) return null
  if (!packed(line)) {
    if (line.allowDecimal) return null
    return (
      <>
        <button type="button" onClick={() => setAsking(true)}
          className={cn(CHIP, 'border border-dashed border-border text-muted-foreground hover:text-foreground')}>
          <Package size={14} /> Set pack
        </button>
        {asking && <PackSize line={line} onClose={() => setAsking(false)} />}
      </>
    )
  }
  const { label } = packGross(line.qty, line.unitPrice, line.packs!)
  return (
    <>
      {label && <span className={cn(CHIP, 'bg-brand-soft text-brand')}>= {label}</span>}
      {line.packs!.map((k) => (
        <button key={k.id} type="button" onClick={() => onQty(line.qty + k.size)} aria-label={`One more ${k.name} of ${line.name}`}
          title={`${k.name} of ${k.size} · ${money(sym, k.price > 0 && k.price < k.size * line.unitPrice ? k.price : k.size * line.unitPrice)}`}
          className={cn(CHIP, 'border border-border bg-card hover:border-brand')}>
          <Plus size={14} /> {k.name}
        </button>
      ))}
    </>
  )
}

/** "How many in one card?" — makes the pack (it follows the unit price, and goes
 * on Items to review) and sets the line to the cards asked for. */
function PackSize({ line, onClose }: { line: CartLine; onClose: () => void }) {
  const { dispatch } = useTill()
  const [name, setName] = useState('Card')
  const [size, setSize] = useState('')
  const [count, setCount] = useState('1')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)
  const ok = num(size) > 1 && Number.isInteger(num(size)) && num(count) > 0 && name.trim() !== ''
  async function save() {
    if (!ok || busy) return
    setBusy(true)
    try {
      const k = await api<TillPack>(`/till/products/${line.productId}/packs`, { json: { name: name.trim(), size } })
      dispatch({ type: 'linePacks', key: line.key, packs: [{ id: k.id, name: k.name, size: num(k.size), price: num(k.sellingPrice) }],
        qty: Math.round(num(count)) * num(k.size) })
      onClose()
    } catch (e) {
      setErr(errMsg(e))
      setBusy(false)
    }
  }
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-[440px]" onKeyDown={(e) => e.key === 'Enter' && (e.preventDefault(), void save())}>
        <DialogHeader>
          <DialogTitle>Sell {line.name} by the pack</DialogTitle>
          <DialogDescription>Read the count off the box or the card in your hand. It is saved for next time.</DialogDescription>
        </DialogHeader>
        <div className="flex flex-wrap gap-1.5">
          {NAMES.map((n) => (
            <button key={n} type="button" aria-pressed={name === n} onClick={() => setName(n)}
              className="rounded-full border border-border px-3 py-1 text-[13px] font-semibold aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground">
              {n}
            </button>
          ))}
        </div>
        <label className="grid gap-1 text-[13px] font-semibold">
          How many in one {name.toLowerCase() || 'pack'}?
          <input autoFocus inputMode="numeric" autoComplete="off" value={size} aria-label="Units in one pack"
            onChange={(e) => setSize(e.target.value.replace(/\D/g, ''))} className={FIELD} placeholder="e.g. 10" />
        </label>
        <label className="grid gap-1 text-[13px] font-semibold">
          How many {packPlural(name.toLowerCase() || 'pack', 2)} to sell now?
          <input inputMode="numeric" autoComplete="off" value={count} aria-label="Packs to sell"
            onChange={(e) => setCount(e.target.value.replace(/\D/g, ''))} className={FIELD} />
        </label>
        {ok && <p className="text-[13px] text-muted-foreground">= {num(count) * num(size)} on the line</p>}
        {err && <p role="alert" className="text-[13px] font-semibold text-danger">{err}</p>}
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>Back</Button>
          <Button disabled={!ok || busy} onClick={() => void save()}>Save & add</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
