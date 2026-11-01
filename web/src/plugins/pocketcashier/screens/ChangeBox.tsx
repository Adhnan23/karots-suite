import type { TillCopy } from '@/api/types.gen'
import { effectivePayments } from '@/features/till/cart/checkout'
import { blankDraft, type Action } from '@/features/till/cart/store'
import { accountTotal, actualChange, balanceDue, changeDue, coinKeep, keptAmount, roundable, saleTotal } from '@/features/till/cart/totals'
import type { TillState } from '@/features/till/cart/types'
import { FIELD } from '@/features/till/ui'
import { dec, money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { changeNotes } from '../change'

/** Change to give, note by note — the notes and coins to hand back. */
export function ChangeNotes({ change, copy }: { change: number; copy: TillCopy }) {
  const ns = changeNotes(change, copy)
  if (!ns.length) return null
  return (
    <ul aria-label="Change by note" className="flex flex-wrap gap-1.5">
      {ns.map((n) => (
        <li key={`${n.isNote}-${n.value}`} className={cn('rounded-[var(--radius-sm)] border border-border bg-card px-2 py-1 text-[12.5px] font-bold tabular-nums', !n.isNote && 'rounded-pill')}>
          {n.qty} × {qtyText(n.value)}
        </li>
      ))}
    </ul>
  )
}

/** Still to pay, or the change: Keep the coins (or all, or an amount) and, with a
 * customer, put what's kept on their account — the PC's change banner. */
export function ChangeBox({ copy, cart, dispatch }: { copy: TillCopy; cart: TillState; dispatch: (a: Action) => void }) {
  const sym = copy.symbol
  const t = saleTotal(cart)
  const ps = effectivePayments(cart, t)
  const due = balanceDue(t, ps)
  const change = changeDue(t, ps)
  const canKeep = roundable(t, ps)
  const kept = canKeep ? keptAmount(cart.keep, change) : 0
  const give = actualChange(change, kept)
  const onAcct = accountTotal(ps)
  const coins = coinKeep(change)
  const cust = copy.customers.find((c) => c.id === cart.customerId)
  const keepBtn = 'rounded-pill border border-border bg-card px-2.5 py-1 text-[12px] font-bold aria-pressed:border-success aria-pressed:bg-success aria-pressed:text-background'
  if (!cart.lines.length) return null

  return (
    <div aria-live="polite" className={cn('mx-3 mt-2 flex flex-col gap-1.5 rounded-[var(--radius-sm)] px-3 py-2.5 text-[13px]',
      due > 0.005 ? 'bg-[color-mix(in_srgb,var(--warning)_16%,transparent)]' : change > 0.005 ? 'bg-[color-mix(in_srgb,var(--success)_16%,transparent)]' : 'bg-muted')}>
      {due > 0.005 ? (
        <>
          <p role="status" className="flex items-baseline justify-between font-bold">Still to pay <b className="font-display text-[22px] text-warning tabular-nums">{money(sym, due)}</b></p>
          {cust && t > 0 && (
            <button type="button" onClick={() => dispatch({ type: 'addTender', payment: { ...blankDraft('credit'), amount: dec(due) } })}
              className="self-start rounded-pill border border-border bg-card px-3 py-1.5 text-[12.5px] font-bold">Put {money(sym, due)} on account</button>
          )}
        </>
      ) : change > 0.005 ? (
        <>
          <p role="status" className="flex items-baseline justify-between font-bold">{t < 0 ? 'Give the customer' : 'Give change'} <b className="font-display text-[22px] text-success tabular-nums">{money(sym, give)}</b></p>
          <ChangeNotes change={give} copy={copy} />
          {canKeep && (
            <div className="flex flex-wrap items-center gap-1.5">
              <button type="button" className={keepBtn} aria-pressed={kept === 0} onClick={() => dispatch({ type: 'keep', value: '' })}>Give exact {money(sym, change)}</button>
              {coins > 0 && coins < change && (
                <button type="button" className={keepBtn} aria-pressed={kept === coins} onClick={() => dispatch({ type: 'keep', value: dec(coins) })}>Keep {money(sym, coins)} (no coins)</button>
              )}
              <button type="button" className={keepBtn} aria-pressed={kept >= change - 0.005} onClick={() => dispatch({ type: 'keep', value: dec(change) })}>Keep all</button>
              <input aria-label="Amount to keep" placeholder="Keep…" inputMode="decimal" autoComplete="off" value={cart.keep}
                onChange={(e) => dispatch({ type: 'keep', value: e.target.value.replace(/[^\d.]/g, '') })} className={cn(FIELD, 'h-7 w-20 px-2 text-[12px]')} />
            </div>
          )}
          {cust && kept > 0 && (
            <label className="flex items-center gap-2 text-[12.5px]">
              <input type="checkbox" checked={cart.roundToAccount} onChange={(e) => dispatch({ type: 'roundToAccount', on: e.target.checked })} />
              Put the kept {money(sym, kept)} on {cust.name}’s account (advance)
            </label>
          )}
        </>
      ) : (
        <p role="status" className="flex items-baseline justify-between font-bold">Paid in full <b className="font-display text-[22px] tabular-nums">{money(sym, t)}</b></p>
      )}
      {onAcct > 0 && <p className="font-semibold text-warning">On account: {money(sym, onAcct)}</p>}
      {cust && onAcct > 0 && num(cust.balance) + onAcct > num(cust.creditLimit) + 0.005 && (
        <p className="text-[12px] font-semibold text-danger">
          {cust.name} would owe {money(sym, num(cust.balance) + onAcct)}, over the {money(sym, cust.creditLimit)} limit. The sale goes through; the owner sees it flagged.
        </p>
      )}
    </div>
  )
}
