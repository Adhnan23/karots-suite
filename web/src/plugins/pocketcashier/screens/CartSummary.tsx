import type { TillCopy } from '@/api/types.gen'
import { Seg } from '@/features/till/bits'
import { unitPriceFor } from '@/features/till/cart/lines'
import { linePacks } from '@/features/till/cart/packs'
import type { Action } from '@/features/till/cart/store'
import { billDiscount, saleTotal } from '@/features/till/cart/totals'
import type { SaleType, TillState } from '@/features/till/cart/types'
import { FIELD } from '@/features/till/ui'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'

/** The sale's price list applied to the cart: plain lines take the product's
 * price for it (the till's unitPriceFor / linePacks); a picked lot, a service, a
 * plugin group or a payout keeps its own price. The phone sends the price it
 * showed, so the lines must follow the switch. */
function priced(cart: TillState, copy: TillCopy, saleType: SaleType): TillState {
  const lines = cart.lines.map((l) => {
    const p = copy.products.find((x) => x.id === l.productId)
    if (!p || l.batchId || l.isService || l.group || l.payout) return l
    return { ...l, unitPrice: unitPriceFor(p, saleType), ...(l.packs && { packs: linePacks(p, saleType) }) }
  })
  return { ...cart, saleType, lines }
}

/** Bill discount and retail / wholesale, at the top of the opened cart. */
export function CartSummary({ cart, dispatch, copy }: { cart: TillState; dispatch: (a: Action) => void; copy: TillCopy }) {
  const sym = copy.symbol
  const off = billDiscount(cart.lines, cart.billDiscount, cart.billDiscountType)
  return (
    <div className="flex flex-col gap-2 border-b border-border px-3 py-2.5">
      <div className="flex items-center gap-2">
        <Seg label="Price list" value={cart.saleType} options={[['retail', 'Retail'], ['wholesale', 'Wholesale']]}
          onChange={(v) => dispatch({ type: 'restore', state: priced(cart, copy, v) })} />
        <span aria-label="Total" className="ml-auto font-display text-base font-extrabold tabular-nums">{money(sym, saleTotal(cart))}</span>
      </div>
      <div className="flex items-center gap-2 text-[13px]">
        <span className="font-semibold text-muted-foreground">Bill discount</span>
        <input aria-label="Bill discount" inputMode="decimal" autoComplete="off" placeholder="0" value={cart.billDiscount}
          onChange={(e) => dispatch({ type: 'billDiscount', value: e.target.value.replace(/[^\d.]/g, '') })} className={cn(FIELD, 'h-9 w-20 text-right tabular-nums')} />
        <Seg label="Bill discount type" value={cart.billDiscountType} options={[['fixed', sym], ['percent', '%']]}
          onChange={(t) => dispatch({ type: 'billDiscount', discountType: t })} />
        {num(cart.billDiscount) > 0 && <span className="ml-auto font-semibold tabular-nums text-success">−{money(sym, off)}</span>}
      </div>
    </div>
  )
}
