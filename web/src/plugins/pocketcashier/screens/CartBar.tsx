import { Minus, Plus, X } from 'lucide-react'
import { useState } from 'react'
import type { TillCopy } from '@/api/types.gen'
import { saleTotal, lineNet } from '@/features/till/cart/totals'
import type { Action } from '@/features/till/cart/store'
import type { TillState } from '@/features/till/cart/types'
import { money, qtyText } from '@/lib/money'
import { CARD } from '../bits'

/** The cart at the foot of Sell: items, customer, total and Pay; tap to see the lines. */
export function CartBar({ cart, dispatch, copy, onPay }: { cart: TillState; dispatch: (a: Action) => void; copy: TillCopy; onPay: () => void }) {
  const [open, setOpen] = useState(false)
  const items = cart.lines.reduce((n, l) => n + l.qty, 0)
  const who = copy.customers.find((c) => c.id === cart.customerId)?.name ?? 'walk-in'
  if (!cart.lines.length) return null
  return (
    <>
      {open && <CartLines cart={cart} dispatch={dispatch} sym={copy.symbol} />}
      <div className="mx-2.5 mb-2 flex items-center gap-2.5 rounded-[var(--radius)] bg-hero bg-[image:var(--hero-art)] px-3 py-2.5 text-hero-foreground shadow-md">
        <button type="button" aria-expanded={open} onClick={() => setOpen(!open)} className="min-w-0 text-left">
          <div className="truncate text-[11.5px] text-hero-muted">{qtyText(items)} {items === 1 ? 'item' : 'items'} · {who}</div>
          <div className="font-display text-xl font-extrabold tabular-nums">{money(copy.symbol, saleTotal(cart))}</div>
        </button>
        <button type="button" onClick={onPay} className="ml-auto rounded-pill bg-brand px-5 py-2.5 text-sm font-extrabold text-brand-foreground">Pay ›</button>
      </div>
    </>
  )
}

export function CartLines({ cart, dispatch, sym }: { cart: TillState; dispatch: (a: Action) => void; sym: string }) {
  return (
    <ul aria-label="Cart" className={`${CARD} mx-3 mb-2 max-h-[40dvh] overflow-y-auto rounded-[var(--radius)] p-0`}>
      {cart.lines.map((l) => (
        <li key={l.key} className="flex items-center gap-2 border-b border-border px-3 py-2 text-[13px] last:border-b-0">
          <span className="flex items-center gap-1.5 font-bold text-muted-foreground">
            <button type="button" aria-label={`Fewer ${l.name}`} onClick={() => dispatch(l.qty <= 1 ? { type: 'remove', key: l.key } : { type: 'setQty', key: l.key, qty: l.qty - 1, lots: [] })}
              className="grid size-[26px] place-items-center rounded-full bg-muted text-foreground">{l.qty <= 1 ? <X size={13} /> : <Minus size={13} />}</button>
            <span className="min-w-6 text-center tabular-nums text-foreground">{qtyText(l.qty)}</span>
            <button type="button" aria-label={`More ${l.name}`} onClick={() => dispatch({ type: 'setQty', key: l.key, qty: l.qty + 1, lots: [] })}
              className="grid size-[26px] place-items-center rounded-full bg-muted text-foreground"><Plus size={13} /></button>
          </span>
          <span className="min-w-0 flex-1 truncate">{l.name}</span>
          <b className="tabular-nums">{money(sym, lineNet(l))}</b>
        </li>
      ))}
    </ul>
  )
}
