import { Minus, Plus, X } from 'lucide-react'
import { useState } from 'react'
import type { TillCopy } from '@/api/types.gen'
import { saleTotal, lineNet } from '@/features/till/cart/totals'
import type { Action } from '@/features/till/cart/store'
import { lotsFor } from '@/features/till/cart/lines'
import type { CartLine, TillState } from '@/features/till/cart/types'
import { money, num, qtyText } from '@/lib/money'
import { CARD } from '../bits'
import { CartSummary } from './CartSummary'
import { LineSheet } from './LineSheet'

/** The cart at the foot of Sell: items, customer, total and Pay; tap to see the lines. */
export function CartBar({ cart, dispatch, copy, onPay }: { cart: TillState; dispatch: (a: Action) => void; copy: TillCopy; onPay: () => void }) {
  const [open, setOpen] = useState(false)
  const items = cart.lines.reduce((n, l) => n + l.qty, 0)
  const who = copy.customers.find((c) => c.id === cart.customerId)?.name ?? 'walk-in'
  if (!cart.lines.length) return null
  return (
    <>
      {open && <CartLines cart={cart} dispatch={dispatch} copy={copy} />}
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

export function CartLines({ cart, dispatch, copy }: { cart: TillState; dispatch: (a: Action) => void; copy: TillCopy }) {
  const [edit, setEdit] = useState<number | null>(null)
  const editing = cart.lines.find((l) => l.key === edit)
  const qty = (l: CartLine, q: number) => dispatch(q <= 0 ? { type: 'remove', key: l.key } : { type: 'setQty', key: l.key, qty: q, lots: lotsFor(copy.priceOptions, l.productId) })
  return (
    <>
    {editing && <LineSheet key={editing.key} line={editing} cart={cart} dispatch={dispatch} copy={copy} onClose={() => setEdit(null)} />}
    <div className={`${CARD} mx-3 mb-2 max-h-[50dvh] overflow-y-auto rounded-[var(--radius)] p-0`}>
    <CartSummary cart={cart} dispatch={dispatch} copy={copy} />
    <ul aria-label="Cart">
      {cart.lines.map((l) => (
        <li key={l.key} className="flex items-center gap-2 border-b border-border px-3 py-2 text-[13px] last:border-b-0">
          <span className="flex items-center gap-1.5 font-bold text-muted-foreground">
            <button type="button" aria-label={`Fewer ${l.name}`} onClick={() => qty(l, l.qty - 1)}
              className="grid size-[26px] place-items-center rounded-full bg-muted text-foreground">{l.qty <= 1 ? <X size={13} /> : <Minus size={13} />}</button>
            <span className="min-w-6 text-center tabular-nums text-foreground">{qtyText(l.qty)}</span>
            <button type="button" aria-label={`More ${l.name}`} onClick={() => qty(l, l.qty + 1)}
              className="grid size-[26px] place-items-center rounded-full bg-muted text-foreground"><Plus size={13} /></button>
          </span>
          <button type="button" aria-label={`Edit ${l.name}`} onClick={() => setEdit(l.key)} className="flex min-w-0 flex-1 items-center gap-2 text-left">
            <span className="min-w-0 flex-1">
              <span className="block truncate">{l.description || l.name}</span>
              {(num(l.discount) > 0 || l.batchLabel || l.serials.some((x) => !x)) && (
                <span className="block truncate text-[11px] text-muted-foreground">
                  {[num(l.discount) > 0 && (l.discountType === 'percent' ? `${l.discount}% off` : `${money(copy.symbol, l.discount)} off each`),
                    l.batchLabel, l.serials.some((x) => !x) && 'serials to enter'].filter(Boolean).join(' · ')}
                </span>
              )}
              {/* The PC asks here; the phone sells, and the PC flags it for a look. */}
              {!l.isService && l.qty > l.stock + 1e-9 && (
                <span className="block truncate text-[11px] font-semibold text-warning">More than the count shows {qtyText(Math.max(0, l.stock))} — it sells, and is flagged</span>
              )}
            </span>
            <b className="tabular-nums">{money(copy.symbol, lineNet(l))}</b>
          </button>
        </li>
      ))}
    </ul>
    </div>
    </>
  )
}
