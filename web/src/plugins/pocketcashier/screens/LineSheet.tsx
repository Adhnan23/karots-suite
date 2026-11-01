import { useState } from 'react'
import type { TillCopy } from '@/api/types.gen'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Seg } from '@/features/till/bits'
import { lotsFor } from '@/features/till/cart/lines'
import type { Action } from '@/features/till/cart/store'
import { lineDiscount, lineGross } from '@/features/till/cart/totals'
import type { CartLine, TillState } from '@/features/till/cart/types'
import { FIELD } from '@/features/till/ui'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { Keypad } from './Keypad'

/** One cart line, opened from the cart: typed quantity (a weight where the unit
 * allows decimals), the line discount, its serials, Remove — the PC's line row
 * as a thumb sheet, on the same reducer actions. */
export function LineSheet({ line, cart, dispatch, copy, onClose }: {
  line: CartLine
  cart: TillState
  dispatch: (a: Action) => void
  copy: TillCopy
  onClose: () => void
}) {
  const [qty, setQty] = useState(qtyText(line.qty))
  const sym = copy.symbol
  const unit = cart.lines.find((l) => l.key === line.key) ?? line
  const gross = lineGross(unit)
  const disc = lineDiscount(unit)
  const discount = (value: string, type = unit.discountType) => dispatch({ type: 'lineDiscount', key: unit.key, discount: value, discountType: type })
  const apply = () => dispatch({ type: 'setQty', key: unit.key, qty: num(qty), lots: lotsFor(copy.priceOptions, unit.productId) })
  // Closing keeps a quantity typed but not set yet (a cashier taps Done, not Set).
  const close = () => {
    if (qty.trim() !== '' && qty !== qtyText(unit.qty) && !unit.fixedQty && !unit.payout) apply()
    onClose()
  }

  return (
    <Sheet open onOpenChange={(o) => !o && close()}>
      <SheetContent side="bottom" className="max-h-[92dvh] gap-3 overflow-y-auto p-4">
        <SheetHeader className="p-0">
          <SheetTitle>{unit.description || unit.name}</SheetTitle>
          <SheetDescription>
            {money(sym, unit.unitPrice)} each{unit.batchLabel && ` · ${unit.batchLabel}`} · {disc > 0 && <s>{money(sym, gross)}</s>} {money(sym, gross - disc)}
          </SheetDescription>
        </SheetHeader>

        {!unit.fixedQty && !unit.payout && (
          <div className="flex flex-col gap-2">
            <div className="flex items-center gap-2">
              <input aria-label="Quantity" inputMode="decimal" autoComplete="off" value={qty} onChange={(e) => setQty(e.target.value.replace(/[^\d.]/g, ''))}
                className={cn(FIELD, 'min-w-0 flex-1 text-center text-xl font-bold tabular-nums')} />
              <button type="button" onClick={apply} className="rounded-[var(--radius)] bg-brand px-4 py-3 text-sm font-extrabold text-brand-foreground">Set quantity</button>
            </div>
            <Keypad value={qty} onChange={setQty} />
          </div>
        )}

        {!unit.payout && !unit.noDiscount && (
          <div className="flex items-center gap-2">
            <span className="text-[13px] font-semibold text-muted-foreground">Discount</span>
            <input aria-label="Discount" inputMode="decimal" autoComplete="off" placeholder="0" value={unit.discount}
              onChange={(e) => discount(e.target.value.replace(/[^\d.]/g, ''))} className={cn(FIELD, 'w-24 text-right tabular-nums')} />
            <Seg label="Discount type" value={unit.discountType} options={[['fixed', sym], ['percent', '%']]} onChange={(t) => discount(unit.discount, t)} />
            {unit.discountType === 'fixed' && <span className="text-[12px] text-faint">each</span>}
          </div>
        )}

        {unit.trackSerial && unit.serials.length > 0 && (
          <div className="grid grid-cols-2 gap-1.5">
            {unit.serials.map((s, i) => (
              <input key={i} aria-label={`Serial ${i + 1}`} placeholder={`Serial ${i + 1}`} autoComplete="off" value={s}
                onChange={(e) => dispatch({ type: 'serial', key: unit.key, index: i, value: e.target.value })} className={FIELD} />
            ))}
          </div>
        )}

        <div className="flex gap-2">
          <button type="button" onClick={() => { dispatch({ type: 'remove', key: unit.key }); onClose() }}
            className="flex-1 rounded-[var(--radius)] border border-border p-3 text-sm font-bold text-destructive">Remove</button>
          <button type="button" onClick={close} className="flex-1 rounded-[var(--radius)] bg-muted p-3 text-sm font-bold">Done</button>
        </div>
      </SheetContent>
    </Sheet>
  )
}
