import { Banknote, BookUser, CreditCard, ScrollText, Smartphone, Wallet, X } from 'lucide-react'
import type { TillCopy } from '@/api/types.gen'
import { chequeMissing } from '@/features/till/cart/checkout'
import { blankDraft, newChequeDraft, type Action } from '@/features/till/cart/store'
import { balanceDue, quickCash, saleTotal } from '@/features/till/cart/totals'
import type { TillState } from '@/features/till/cart/types'
import { ChequeFields } from '@/features/till/pay/ChequeFields'
import { FIELD } from '@/features/till/ui'
import { METHOD_LABEL } from '@/lib/payMethods'
import { dec, money, num, r2, short } from '@/lib/money'
import { cn } from '@/lib/utils'
import { slots } from '@/plugins/slots'
import { GLASS } from '../bits'

/** The PC till's tender on the phone: pick a method, key or tap the amount, Add —
 * as many payments as the sale needs (cash + card, part on account…), each
 * removable. Plugin tenders only live, unless they work from the copy. */
export function PayTenders({ copy, cart, dispatch, live, setError }: {
  copy: TillCopy
  cart: TillState
  dispatch: (a: Action) => void
  live: boolean
  setError: (e: string) => void
}) {
  const card = copy.places.filter((p) => p.kind === 'card' && p.cashierAccess)
  const bank = copy.places.filter((p) => p.kind === 'bank' && p.cashierAccess)
  const d = cart.draft
  const due = balanceDue(saleTotal(cart), cart.tenders)
  const methods = [
    { value: 'cash', label: 'Cash', Icon: Banknote },
    ...(card.length ? [{ value: 'card', label: 'Card', Icon: CreditCard }] : []),
    ...(bank.length ? [{ value: 'online', label: 'Online', Icon: Smartphone }] : []),
    { value: 'cheque', label: 'Cheque', Icon: ScrollText },
    { value: 'credit', label: 'On account', Icon: BookUser },
    ...copy.tenderMethods.filter((m) => !m.hidden).flatMap((m) => {
      const slot = slots.tillTenders.find((x) => x.value === m.value)
      return live || slot?.offline ? [{ value: m.value, label: m.label, Icon: slot?.Icon ?? Wallet }] : []
    }),
  ]
  const accounts = d.method === 'card' ? card : d.method === 'online' ? bank : []
  const Detail = slots.tillTenders.find((x) => x.value === d.method)?.Detail
  const label = (m: string) => METHOD_LABEL[m] ?? copy.tenderMethods.find((t) => t.value === m)?.label ?? m
  const draft = (patch: Partial<typeof d>) => { setError(''); dispatch({ type: 'draft', patch }) }

  function choose(m: string) {
    const first = m === 'card' ? card[0] : m === 'online' ? bank[0] : undefined
    draft({ method: m, accountId: first ? String(first.id) : '', meta: {}, cheque: m === 'cheque' ? (d.cheque ?? newChequeDraft()) : undefined })
  }
  const chequeReady = () => {
    const miss = d.method === 'cheque' ? chequeMissing(d.cheque) : ''
    setError(miss)
    return !miss
  }
  const commit = () => (!d.method ? setError('Choose a payment method') : chequeReady() && dispatch({ type: 'commitDraft' }))
  const notes = (copy.denominations ?? []).filter((x) => x.isNote).map((x) => num(x.value))
  const chips = d.method === 'cash' ? quickCash(due, notes.length ? notes : undefined) : [r2(due)]

  return (
    <div className="mx-3 flex flex-col gap-2">
      <div role="group" aria-label="Payment method" className="grid grid-cols-4 gap-1.5">
        {methods.map(({ value, label: text, Icon }) => {
          const needsCustomer = (value === 'credit' || value === 'cheque') && cart.customerId === null
          return (
            <button key={value} type="button" aria-pressed={d.method === value} disabled={needsCustomer} onClick={() => choose(value)}
              title={needsCustomer ? (value === 'cheque' ? 'Pick a customer to take a cheque' : 'Pick a customer to sell on account') : undefined}
              className={cn('flex flex-col items-center gap-0.5 rounded-[var(--radius-sm)] border border-border bg-card px-0.5 py-2 text-[11.5px] font-bold disabled:opacity-40 aria-pressed:border-transparent aria-pressed:bg-selected aria-pressed:text-selected-foreground', GLASS)}>
              <Icon size={18} />{text}
            </button>
          )
        })}
      </div>
      {accounts.length > 1 && (
        <select aria-label="Account" value={d.accountId} onChange={(e) => draft({ accountId: e.target.value })} className={cn(FIELD, 'appearance-auto')}>
          {accounts.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
        </select>
      )}
      {(d.method === 'card' || d.method === 'online') && (
        <input aria-label="Reference" placeholder="Reference (optional)" autoComplete="off" value={d.reference} onChange={(e) => draft({ reference: e.target.value })} className={FIELD} />
      )}
      {d.method === 'cheque' && d.cheque && <ChequeFields value={d.cheque} onChange={(cheque) => draft({ cheque })} />}
      {Detail && <Detail meta={d.meta} setMeta={(meta) => draft({ meta })} />}
      <div className="flex gap-1.5">
        <input aria-label="Amount tendered" inputMode="decimal" autoComplete="off" placeholder={due > 0 ? money(copy.symbol, due) : 'Amount'} value={d.amount}
          onChange={(e) => draft({ amount: e.target.value.replace(/[^\d.]/g, '') })} className={cn(FIELD, 'min-w-0 flex-1 font-display text-lg font-bold tabular-nums')} />
        <button type="button" disabled={!(num(d.amount) > 0)} onClick={commit}
          className="rounded-[var(--radius-sm)] bg-brand-soft px-4 text-sm font-extrabold text-foreground disabled:opacity-50">Add payment</button>
      </div>
      {d.method !== '' && due > 0 && (
        <div role="group" aria-label={d.method === 'cash' ? 'Cash handed over' : 'Pay the exact amount'} className="grid grid-cols-4 gap-1.5">
          {chips.map((v, i) => (
            <button key={v} type="button"
              onClick={() => chequeReady() && dispatch({ type: 'addTender', payment: d.method === 'cash' ? { ...blankDraft('cash'), amount: dec(v) } : { ...d, amount: dec(v) } })}
              className={cn('flex h-9 min-w-0 items-center justify-center gap-1 whitespace-nowrap rounded-pill border px-1.5 text-[12.5px] font-bold tabular-nums',
                i === 0 ? 'col-span-2 border-[color-mix(in_srgb,var(--brand)_45%,transparent)] bg-brand-soft' : 'border-border bg-card')}>
              {i === 0 && <span className="text-[11px] font-semibold text-muted-foreground">{cart.tenders.length ? 'Remaining' : 'Exact'}</span>}
              {short(v)}
            </button>
          ))}
        </div>
      )}
      {cart.tenders.length > 0 && (
        <ul aria-label="Payments" className="flex flex-col gap-1">
          {cart.tenders.map((p, i) => (
            <li key={`${i}-${p.method}-${p.amount}`} className="flex items-center gap-2 rounded-[var(--radius-sm)] bg-muted px-2.5 py-1.5 text-[13px]">
              {label(p.method)}
              {p.cheque && <small className="min-w-0 truncate text-muted-foreground">{[p.cheque.bank, `#${p.cheque.number}`].join(' · ')}</small>}
              {p.accountId && <small className="truncate text-muted-foreground">{[...card, ...bank].find((a) => String(a.id) === p.accountId)?.name}</small>}
              <b className="ml-auto tabular-nums">{money(copy.symbol, p.amount)}</b>
              <button type="button" aria-label={`Remove ${label(p.method)} payment`} onClick={() => dispatch({ type: 'removeTender', index: i })}
                className="grid size-6 place-items-center text-faint"><X size={14} /></button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
