import { useSuspenseQuery } from '@tanstack/react-query'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery } from '../api'
import { Kbd, Seg } from '../bits'
import { billDiscount, goodsSubtotal, itemDiscountTotal, payoutTotal, saleTotal, taxTotal } from '../cart/totals'
import { useTill } from '../TillProvider'
import { FIELD } from '../ui'

export function Summary() {
  const cfg = useSuspenseQuery(configQuery).data
  const { state, dispatch } = useTill()
  const sym = cfg.symbol
  const ls = state.lines
  const idisc = itemDiscountTotal(ls)
  const bdisc = billDiscount(ls, state.billDiscount, state.billDiscountType)
  const tax = taxTotal(ls)
  const given = payoutTotal(ls)
  const t = saleTotal(state)
  return (
    <div className="flex flex-col gap-1.5 text-[13.5px]">
      <div className="flex justify-between text-muted-foreground"><span>Subtotal</span><span className="tabular-nums">{money(sym, goodsSubtotal(ls))}</span></div>
      {idisc > 0 && <div className="flex justify-between text-success"><span>Item discounts</span><span className="tabular-nums">−{money(sym, idisc)}</span></div>}
      <div className="flex items-center justify-between gap-2 text-muted-foreground">
        <span className="flex items-center gap-1.5">
          Bill discount
          <input id="till-bill-discount" aria-label="Bill discount" inputMode="decimal" placeholder="0" autoComplete="off" value={state.billDiscount}
            onChange={(e) => dispatch({ type: 'billDiscount', value: e.target.value.replace(/[^\d.]/g, '') })}
            className={cn(FIELD, 'h-7 w-16 px-2 text-right text-[13px] tabular-nums')} />
          <Seg label="Bill discount type" value={state.billDiscountType} options={[['fixed', sym], ['percent', '%']]}
            onChange={(v) => dispatch({ type: 'billDiscount', discountType: v })} />
          <Kbd>F9</Kbd>
        </span>
        <span className="tabular-nums">{bdisc > 0 ? `−${money(sym, bdisc)}` : '—'}</span>
      </div>
      {tax > 0 && <div className="flex justify-between text-muted-foreground"><span>Tax</span><span className="tabular-nums">{money(sym, tax)}</span></div>}
      {given > 0 && <div className="flex justify-between text-warning"><span>Given to the customer</span><span className="tabular-nums">−{money(sym, given)}</span></div>}
      <div className="mt-1 flex items-baseline justify-between border-t border-border pt-2">
        <span className="font-semibold text-muted-foreground">{t < 0 ? 'Give the customer' : 'Total'}</span>
        <b className={cn('font-display text-[32px] font-extrabold tracking-tight tabular-nums', t < 0 && 'text-warning')}>{money(sym, Math.abs(t))}</b>
      </div>
    </div>
  )
}
