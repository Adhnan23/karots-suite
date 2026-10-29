import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { Banknote, BookUser, CreditCard, ScrollText, Smartphone, Wallet, X } from 'lucide-react'
import { AnimatePresence, motion } from 'motion/react'
import { tx } from '@/theme/motion'
import { toast } from 'sonner'
import { dec, money, num, r2, short } from '@/lib/money'
import { cn } from '@/lib/utils'
import { TouchKeys } from './TouchKeys'
import { slots } from '@/plugins/slots'
import { Button } from '@/components/ui/button'
import { configQuery, drawerQuery, lockersOf } from '../api'
import { blankDraft } from '../cart/store'
import { balanceDue, LKR_NOTES, quickCash, saleTotal } from '../cart/totals'
import { useCheckoutCtx } from '../checkout/CheckoutProvider'
import { useTill } from '../TillProvider'
import { FIELD } from '../ui'
import { MethodBadge } from '@/components/admin/MethodBadge'
import { today } from '@/features/money/cheques/queries'
import { METHOD_LABEL } from '@/lib/payMethods'
import { ChequeFields } from './ChequeFields'

export { METHOD_LABEL }

export function Tender() {
  const cfg = useSuspenseQuery(configQuery).data
  const drawer = useQuery(drawerQuery).data
  const { state, dispatch } = useTill()
  const co = useCheckoutCtx()
  // The shop's notes (Denominations) — what a customer can hand over.
  const notes = (drawer?.denominations ?? []).filter((x) => x.isNote).map((x) => num(x.value))
  const coins = (drawer?.denominations ?? []).filter((x) => !x.isNote).map((x) => num(x.value))
  const card = lockersOf(drawer, 'card')
  const bank = lockersOf(drawer, 'bank')
  const d = state.draft
  const due = balanceDue(saleTotal(state), state.tenders)
  // Card/online buttons appear only once the owner opened a cashier-accessible card/bank account.
  const methods = [
    { value: 'cash', label: 'Cash', Icon: Banknote },
    ...(card.length ? [{ value: 'card', label: 'Card', Icon: CreditCard }] : []),
    ...(bank.length ? [{ value: 'online', label: 'Online', Icon: Smartphone }] : []),
    { value: 'cheque', label: 'Cheque', Icon: ScrollText },
    { value: 'credit', label: 'On account', Icon: BookUser },
    ...cfg.tenderMethods.filter((m) => !m.hidden).map((m) => ({ value: m.value, label: m.label, Icon: slots.tillTenders.find((t) => t.value === m.value)?.Icon ?? Wallet })),
  ]
  const accounts = d.method === 'card' ? card : d.method === 'online' ? bank : []
  const PluginDetail = slots.tillTenders.find((t) => t.value === d.method)?.Detail
  const label = (m: string) => METHOD_LABEL[m] ?? cfg.tenderMethods.find((t) => t.value === m)?.label ?? m

  function choose(m: string) {
    const first = m === 'card' ? card[0] : m === 'online' ? bank[0] : undefined
    const cheque = m === 'cheque' ? (d.cheque ?? { bank: '', number: '', date: today(), note: '' }) : undefined
    dispatch({ type: 'draft', patch: { method: m, accountId: first ? String(first.id) : '', meta: {}, cheque } })
    requestAnimationFrame(() => document.getElementById('till-amount')?.focus())
  }
  const commit = () => (d.method ? dispatch({ type: 'commitDraft' }) : void toast.error('Choose a payment method'))

  return (
    <div className="flex flex-col gap-2.5">
      <div className="grid grid-cols-4 gap-1.5" role="group" aria-label="Payment method">
        {methods.map(({ value, label: text, Icon }) => {
          const needsCustomer = (value === 'credit' || value === 'cheque') && state.customerId === null
          return (
            <button key={value} type="button" aria-pressed={d.method === value} disabled={needsCustomer}
              title={needsCustomer ? (value === 'cheque' ? 'Pick a customer to take a cheque' : 'Pick a customer to sell on account') : undefined} onClick={() => choose(value)}
              className="flex flex-col items-center gap-1 rounded-[var(--radius-sm)] border border-border bg-muted px-1 py-2 text-[12px] font-bold disabled:opacity-40 aria-pressed:border-brand aria-pressed:bg-brand aria-pressed:text-brand-foreground">
              <Icon size={18} />{text}
            </button>
          )
        })}
      </div>
      {accounts.length > 1 && (
        <select aria-label="Account" value={d.accountId} onChange={(e) => dispatch({ type: 'draft', patch: { accountId: e.target.value } })} className={cn(FIELD, 'appearance-auto')}>
          {accounts.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
        </select>
      )}
      {(d.method === 'card' || d.method === 'online') && (
        <input aria-label="Reference" placeholder="Reference (optional)" autoComplete="off" value={d.reference}
          onChange={(e) => dispatch({ type: 'draft', patch: { reference: e.target.value } })} className={FIELD} />
      )}
      {d.method === 'cheque' && d.cheque && <ChequeFields value={d.cheque} onChange={(cheque) => dispatch({ type: 'draft', patch: { cheque } })} />}
      {PluginDetail && <PluginDetail meta={d.meta} setMeta={(meta) => dispatch({ type: 'draft', patch: { meta } })} />}
      <div className="flex gap-1.5">
        <input id="till-amount" aria-label="Amount tendered" inputMode="decimal" autoComplete="off"
          placeholder={due > 0 ? money(cfg.symbol, due) : 'Amount'} value={d.amount}
          onChange={(e) => dispatch({ type: 'draft', patch: { amount: e.target.value.replace(/[^\d.]/g, '') } })}
          onKeyDown={(e) => {
            if (e.key !== 'Enter') return
            e.preventDefault()
            e.stopPropagation()
            if (num(d.amount) > 0) commit()
            else void co.run() // F10 → amount → Enter: an empty box completes the sale
          }}
          className={cn(FIELD, 'min-w-0 flex-1 font-display text-lg font-bold tabular-nums')} />
        <Button variant="secondary" className="h-[var(--control-h)]" title="Fill the remaining balance"
          onClick={() => dispatch({ type: 'draft', patch: { amount: due > 0 ? dec(due) : '' } })}>Fill</Button>
        <Button className="h-[var(--control-h)]" disabled={!(num(d.amount) > 0)} onClick={commit}>Add</Button>
      </div>
      {d.method !== '' && due > 0 && (
        // Cash: what a customer would hand over in notes. Card, online, on account…:
        // just the exact due (they can't pay more), with the draft's account / details.
        <div className="grid grid-cols-4 gap-1.5" role="group" aria-label={d.method === 'cash' ? 'Cash handed over' : 'Pay the exact amount'}>
          {(d.method === 'cash' ? quickCash(due, notes.length ? notes : undefined).slice(0, 7) : [r2(due)]).map((v, i) => (
            <button key={v} type="button"
              onClick={() => dispatch({ type: 'addTender', payment: d.method === 'cash' ? { ...blankDraft('cash'), amount: dec(v) } : { ...d, amount: dec(v) } })}
              className={cn('flex h-9 min-w-0 items-center justify-center gap-1.5 whitespace-nowrap rounded-pill border px-2 text-[13px] font-bold tabular-nums',
                i === 0 ? 'col-span-2 border-[color-mix(in_srgb,var(--brand)_45%,transparent)] bg-brand-soft hover:bg-[color-mix(in_srgb,var(--brand)_22%,transparent)]' : 'border-border bg-card hover:bg-muted')}>
              {i === 0 && <span className="text-[11.5px] font-semibold text-muted-foreground">{state.tenders.length ? 'Remaining' : 'Exact'}</span>}
              {short(v)}
            </button>
          ))}
        </div>
      )}
      <TouchKeys cash={d.method === 'cash' && due > 0} notes={notes.length ? notes : LKR_NOTES} coins={coins} amount={d.amount}
        onAmount={(amount) => dispatch({ type: 'draft', patch: { amount } })} />
      <div className="flex flex-col gap-1">
        <AnimatePresence initial={false}>
          {state.tenders.map((p, i) => (
            <motion.div key={`${i}-${p.method}-${p.amount}`} initial={{ y: -4, opacity: 0.3 }} animate={{ y: 0, opacity: 1 }} exit={{ opacity: 0 }} transition={tx()}
              className="flex items-center gap-2 rounded-[var(--radius-sm)] bg-muted px-2.5 py-1.5 text-[13px]">
              {p.method === 'cheque' ? <MethodBadge method="cheque" /> : label(p.method)}
              {p.cheque && <small className="min-w-0 truncate text-muted-foreground">{[p.cheque.bank, `#${p.cheque.number}`, p.cheque.date].filter(Boolean).join(' · ')}</small>}
              {p.accountId && <small className="text-muted-foreground">{[...card, ...bank].find((a) => String(a.id) === p.accountId)?.name}</small>}
              {p.reference && <small className="text-muted-foreground">#{p.reference}</small>}
              <b className="ml-auto tabular-nums">{money(cfg.symbol, p.amount)}</b>
              <button type="button" aria-label={`Remove ${label(p.method)} payment`} onClick={() => dispatch({ type: 'removeTender', index: i })}
                className="text-faint hover:text-danger"><X size={14} /></button>
            </motion.div>
          ))}
        </AnimatePresence>
      </div>
    </div>
  )
}
