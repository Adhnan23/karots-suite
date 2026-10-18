import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { dec, money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, customersQuery, drawerQuery } from '../api'
import {
  accountTotal, actualChange, balanceDue, changeDue, coinKeep, keptAmount, notesFor, paidTotal, roundable, saleTotal,
} from '../cart/totals'
import { useTill } from '../TillProvider'
import { FIELD } from '../ui'

/** Still to pay / change to give (with Keep) / paid in full — plus account and limit cues. */
export function ChangeBanner() {
  const cfg = useSuspenseQuery(configQuery).data
  const { state, dispatch } = useTill()
  const drawer = useQuery(drawerQuery).data
  const cust = (useQuery(customersQuery).data ?? []).find((c) => c.id === state.customerId)
  const sym = cfg.symbol
  const t = saleTotal(state)
  const pays = [...state.tenders, ...(num(state.draft.amount) > 0 ? [state.draft] : [])]
  const due = balanceDue(t, pays)
  const change = changeDue(t, pays)
  const canKeep = roundable(t, pays)
  const kept = canKeep ? keptAmount(state.keep, change) : 0
  const give = actualChange(change, kept)
  const onAcct = accountTotal(pays)
  const avail = cust ? num(cust.creditLimit) - num(cust.outstanding) : null
  const over = cust && avail !== null && onAcct > 0.005 && onAcct > avail + 0.005
  const coins = coinKeep(change)
  const notes = notesFor(give, (drawer?.denominations ?? []).map((d) => num(d.value)))
  const keepBtn = 'rounded-full border border-border bg-card px-2.5 py-1 text-[12px] font-bold aria-pressed:border-success aria-pressed:bg-success aria-pressed:text-background'

  let tone = 'bg-muted'
  let body
  if (!state.lines.length) {
    body = <p className="font-bold">Nothing to pay</p>
  } else if (due > 0) {
    tone = 'bg-[color-mix(in_srgb,var(--warning)_16%,transparent)]'
    body = (
      <>
        <p className="flex items-baseline justify-between font-bold">Still to pay <b className="font-display text-[22px] text-warning tabular-nums">{money(sym, due)}</b></p>
        <p className="text-muted-foreground">
          {pays.length ? `Paid ${money(sym, paidTotal(pays) + onAcct)} of ${money(sym, t)} · add another payment to split`
            : state.draft.method === 'cash' ? 'Press Enter for exact cash' : 'Pick how they pay'}
        </p>
      </>
    )
  } else if (change > 0) {
    tone = 'bg-[color-mix(in_srgb,var(--success)_16%,transparent)]'
    body = (
      <>
        <p className="flex items-baseline justify-between font-bold">{t < 0 ? 'Give the customer' : 'Change to give'} <b className="font-display text-[22px] text-success tabular-nums">{money(sym, give)}</b></p>
        {notes.length > 0 && <p className="text-muted-foreground tabular-nums">{notes.map((n) => `${n.qty} × ${qtyText(n.value)}`).join(' · ')}</p>}
        {canKeep && (
          <div className="flex flex-wrap items-center gap-1.5">
            <button type="button" className={keepBtn} aria-pressed={kept === 0} onClick={() => dispatch({ type: 'keep', value: '' })}>Give exact {money(sym, change)}</button>
            {coins > 0 && coins < change && (
              <button type="button" className={keepBtn} aria-pressed={kept === coins} onClick={() => dispatch({ type: 'keep', value: dec(coins) })}>Keep {money(sym, coins)} (no coins)</button>
            )}
            <button type="button" className={keepBtn} aria-pressed={kept >= change - 0.005} onClick={() => dispatch({ type: 'keep', value: dec(change) })}>Keep all</button>
            <input aria-label="Amount to keep" placeholder="Keep…" inputMode="decimal" autoComplete="off" value={state.keep}
              onChange={(e) => dispatch({ type: 'keep', value: e.target.value.replace(/[^\d.]/g, '') })} className={cn(FIELD, 'h-7 w-20 px-2 text-[12px]')} />
          </div>
        )}
        {cust && kept > 0 && (
          <label className="flex items-center gap-2 text-[12.5px]">
            <input type="checkbox" checked={state.roundToAccount} onChange={(e) => dispatch({ type: 'roundToAccount', on: e.target.checked })} />
            Put the kept {money(sym, kept)} on {cust.name}’s account (advance)
          </label>
        )}
      </>
    )
  } else {
    body = <p className="flex items-baseline justify-between font-bold">Paid in full <b className="font-display text-[22px] tabular-nums">{money(sym, paidTotal(pays) + onAcct)}</b></p>
  }

  return (
    <div className={cn('flex flex-col gap-1.5 rounded-[var(--radius-sm)] px-3 py-2.5 text-[13px] transition-colors', tone)} aria-live="polite">
      {body}
      {onAcct > 0 && <p className="font-semibold text-warning">On account: {money(sym, onAcct)}</p>}
      {over && cust && (
        <p className="text-[12px] font-semibold text-danger">
          {cust.name} would owe {money(sym, num(cust.outstanding) + onAcct)}, over the {money(sym, cust.creditLimit)} limit.
          {cfg.canManageCredit ? ' You can approve it when you complete.' : ' A manager must approve.'}
        </p>
      )}
      {drawer?.summary.session && <p className="text-[12px] text-faint">In drawer {money(sym, drawer.summary.expected)}</p>}
    </div>
  )
}
