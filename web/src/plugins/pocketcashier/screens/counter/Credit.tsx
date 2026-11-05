import { Banknote, CreditCard, ScrollText, Search, Smartphone, Wallet } from 'lucide-react'
import { useState } from 'react'
import type { CopyCustomer } from '@/api/types.gen'
import { blankCheque, ChequeLines, chequesReady, chequesTotal, toChequeLines, type ChequeDraftLine } from '@/components/admin/ChequeLines'
import { Button } from '@/components/ui/button'
import { Seg } from '@/features/till/bits'
import { printDebtSlip, printMoneySlip } from '@/features/till/print'
import { FIELD } from '@/features/till/ui'
import { dec, money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { slots } from '@/plugins/slots'
import { CARD } from '../../bits'
import { drawerOpen } from '../../drawer'
import { sendCall } from '../../sync'
import { AskPrint } from '../AskPrint'
import { CounterShell, type CounterProps } from './CounterShell'

/** Customers with a balance either way: take what they owe, or hand back an
 * advance. Works from the copy; the PC settles it when the PC has it. */
export function Credit(p: CounterProps) {
  const [q, setQ] = useState('')
  const [id, setId] = useState<number | null>(null)
  const [notice, setNotice] = useState('')
  const needle = q.trim().toLowerCase()
  const rows = p.copy.customers.filter((c) => num(c.balance) !== 0 && (!needle || c.name.toLowerCase().includes(needle) || c.phone.includes(needle)))
  const cur = p.copy.customers.find((c) => c.id === id)
  if (cur) return <CreditForm {...p} c={cur} onDone={(n) => { setNotice(n); setId(null); p.onSent() }} onBack={() => setId(null)} />
  return (
    <CounterShell title="Credit" sub={p.live ? undefined : 'Balances as of the last sync'} onBack={p.onBack}>
      {notice && <p role="status" className="px-1 text-sm font-semibold text-success">{notice}</p>}
      <div className="relative">
        <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
        <input aria-label="Find a customer" placeholder="Name or phone" autoComplete="off" value={q} onChange={(e) => setQ(e.target.value)} className={cn(FIELD, 'h-12 w-full pl-9')} />
      </div>
      {rows.length === 0 && <p className="m-auto p-8 text-center text-faint">{needle ? `No customer matches “${q}”.` : 'No one owes the shop.'}</p>}
      {rows.map((c) => (
        <button key={c.id} type="button" onClick={() => { setNotice(''); setId(c.id) }} className={cn(CARD, 'flex min-h-14 items-center gap-3 px-4 py-2.5 text-left')}>
          <span className="min-w-0 flex-1"><b className="block truncate text-[15px]">{c.name}</b><small className="text-muted-foreground tabular-nums">{c.phone || 'no phone'}</small></span>
          <span className="shrink-0 text-right"><b className={cn('block tabular-nums', num(c.balance) > 0 ? 'text-warning' : 'text-success')}>{money(p.copy.symbol, Math.abs(num(c.balance)))}</b><small className="text-muted-foreground">{num(c.balance) > 0 ? 'owes' : 'advance'}</small></span>
        </button>
      ))}
    </CounterShell>
  )
}

type Mode = 'pay' | 'refund'

function CreditForm({ who, copy, out, live, c, onDone, onBack }: CounterProps & { c: CopyCustomer; onDone: (notice: string) => void }) {
  const sym = copy.symbol
  const bal = num(c.balance)
  const advance = bal < 0 ? -bal : 0
  const [mode, setMode] = useState<Mode>('pay')
  const [amount, setAmount] = useState(bal > 0 ? dec(bal) : '')
  const [method, setMethod] = useState('cash')
  const [account, setAccount] = useState('')
  const [reference, setReference] = useState('')
  const [note, setNote] = useState('')
  const [meta, setMeta] = useState<Record<string, string>>({})
  const [cheques, setCheques] = useState<ChequeDraftLine[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [ask, setAsk] = useState<{ print: () => void; notice: string } | null>(null)
  const places = (kind: string) => copy.places.filter((pl) => pl.kind === kind && pl.cashierAccess)
  const accounts = method === 'card' ? places('card') : method === 'online' ? places('bank') : []
  const methods = [
    { v: 'cash', label: 'Cash', Icon: Banknote },
    ...(places('card').length ? [{ v: 'card', label: 'Card', Icon: CreditCard }] : []),
    ...(places('bank').length ? [{ v: 'online', label: 'Online', Icon: Smartphone }] : []),
    { v: 'cheque', label: 'Cheque', Icon: ScrollText },
    ...copy.tenderMethods.filter((t) => t.onPayments).flatMap((t) => {
      const slot = slots.tillTenders.find((x) => x.value === t.value)
      return live || slot?.offline ? [{ v: t.value, label: t.label, Icon: slot?.Icon ?? Wallet }] : []
    }),
  ]
  const Detail = slots.tillTenders.find((t) => t.value === method)?.Detail
  const a = num(amount)
  const after = mode === 'pay' ? bal - a : bal + a
  const byCheque = mode === 'pay' && method === 'cheque'
  const usesDrawer = mode === 'refund' || method === 'cash'
  const open = drawerOpen(copy, who.id, out)
  const ready = a > 0 && (!usesDrawer || open) && (!byCheque || (chequesReady(cheques, 'in') && chequesTotal(cheques) === a))

  async function submit(e: { preventDefault(): void }) {
    e.preventDefault()
    setBusy(true)
    setError('')
    const shown = dec(a)
    const r = mode === 'pay'
      ? await sendCall(who.id, {
        method: 'POST', path: `/api/v1/till/customers/${c.id}/payment`, label: `Credit from ${c.name}: ${money(sym, a)}`,
        cashIn: method === 'cash' ? amount : undefined, saw: { customerId: c.id, balance: c.balance },
        body: { note, amount, method, accountId: num(account || accounts[0]?.id), reference, deferDrawerKick: true,
          meta: Object.keys(meta).length ? meta : undefined, cheques: byCheque ? toChequeLines(cheques) : undefined },
      })
      : await sendCall(who.id, {
        method: 'POST', path: `/api/v1/till/customers/${c.id}/refund`, label: `Advance refund to ${c.name}: ${money(sym, a)}`,
        cashOut: shown, saw: { customerId: c.id, balance: c.balance }, body: { amount: shown, deferDrawerKick: true },
      })
    setBusy(false)
    if (r?.status === 'failed') return setError(r.note || 'The PC could not do this')
    const verb = mode === 'pay' ? `${money(sym, a)} collected from ${c.name}` : `Give ${c.name} ${money(sym, a)}: advance refunded`
    if (!r) return onDone(`${verb} on this phone. It sends when the PC is back; no receipt yet.`)
    const notice = `${verb}. The receipt prints at the counter.`
    const print = r.resultId ? () => void (mode === 'pay' ? printDebtSlip(r.resultId!) : printMoneySlip(r.resultId!)).catch(() => {}) : null
    if (print && copy.askToPrint) return setAsk({ print, notice })
    print?.()
    onDone(notice)
  }

  return (
    <CounterShell title={c.name} sub={c.phone || undefined} onBack={onBack}>
      <form onSubmit={submit} autoComplete="off" className="flex flex-col gap-3">
        {advance > 0 && (
          <Seg label="Action" value={mode} onChange={(m) => { setMode(m); setAmount(m === 'refund' ? dec(advance) : '') }} options={[['pay', 'Take payment'], ['refund', 'Refund advance']]} />
        )}
        <div className={cn(CARD, 'px-4 py-3')}>
          <small className="text-muted-foreground">{bal >= 0 ? 'Owes now' : 'Advance now'}{live ? '' : ' (last sync)'}</small>
          <b className="block font-display text-2xl tabular-nums">{money(sym, Math.abs(bal))}</b>
        </div>
        <input aria-label="Amount" inputMode="decimal" placeholder={money(sym, 0)} value={amount} onChange={(e) => setAmount(e.target.value.replace(/[^\d.]/g, ''))}
          className={cn(FIELD, 'h-14 w-full font-display text-2xl font-bold tabular-nums')} />
        {mode === 'pay' && a > 0 && after < -0.004 && <p className="px-1 text-[13px] text-muted-foreground">{money(sym, -after)} becomes an advance</p>}
        {mode === 'pay' && methods.length > 1 && (
          <div role="radiogroup" aria-label="Paid by" className="grid grid-cols-3 gap-1.5">
            {methods.map(({ v, label, Icon }) => (
              <button key={v} type="button" role="radio" aria-checked={method === v} onClick={() => { setMethod(v); setAccount(''); setMeta({}); setCheques(v === 'cheque' ? [blankCheque(amount)] : []) }}
                className="flex min-h-14 flex-col items-center justify-center gap-1 rounded-[var(--radius-sm)] border border-border bg-muted py-2 text-[12px] font-semibold aria-checked:border-primary aria-checked:bg-primary aria-checked:text-primary-foreground">
                <Icon size={18} />{label}
              </button>
            ))}
          </div>
        )}
        {byCheque && <ChequeLines dir="in" lines={cheques} onChange={setCheques} sym={sym} want={a} />}
        {mode === 'pay' && Detail && <Detail meta={meta} setMeta={setMeta} />}
        {accounts.length > 1 && (
          <select aria-label="Account" value={account || String(accounts[0].id)} onChange={(e) => setAccount(e.target.value)} className={cn(FIELD, 'h-12 w-full')}>
            {accounts.map((l) => <option key={l.id} value={l.id}>{l.name}</option>)}
          </select>
        )}
        {mode === 'pay' && method !== 'cash' && method !== 'cheque' && <input aria-label="Reference" placeholder="Reference (optional)" value={reference} onChange={(e) => setReference(e.target.value)} className={cn(FIELD, 'h-12 w-full')} />}
        {mode === 'pay' && <input aria-label="Note" placeholder="Note (optional)" value={note} onChange={(e) => setNote(e.target.value)} className={cn(FIELD, 'h-12 w-full')} />}
        {usesDrawer && !open && a > 0 && <p className="px-1 text-[13px] font-semibold text-warning">Open the drawer first: this is cash.</p>}
        {error && <p role="alert" className="px-1 text-[13px] font-semibold text-danger">{error}</p>}
        <Button type="submit" size="lg" disabled={busy || !ready} className="h-12 text-[15px]">
          {mode === 'pay' ? `Collect ${a > 0 ? money(sym, a) : ''}` : `Refund ${a > 0 ? money(sym, a) : ''} from the drawer`}
        </Button>
      </form>
      {ask && <AskPrint onPrint={ask.print} onClose={() => onDone(ask.notice)} />}
    </CounterShell>
  )
}
