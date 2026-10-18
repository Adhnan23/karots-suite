import { useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { FIELD } from '@/features/till/ui'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import type { TillApi } from '../slots'
import { Amount, BelowZero, Pick } from './bits'
import { LABEL, floatItem } from './words'
import { addBill, addCashout, addDeposit, chargeFor, goesBelow, inCart } from './till'
import type { TillData } from './types'

export type MoneyMode = 'billpay' | 'deposit' | 'cashout'

const WORDS: Record<MoneyMode, { account: string; amount: string; add: string; empty: string }> = {
  billpay: { account: 'Paid from', amount: 'Bill amount', add: 'Add bill', empty: 'No bank account open to the till and no money device yet.' },
  deposit: { account: 'Send from', amount: 'Amount to send', add: 'Add deposit', empty: 'No money device yet — add one in Apps → Reload & Bills.' },
  cashout: { account: 'Money comes into', amount: 'Cash to give', add: 'Add cash out', empty: 'No bank account open to the till and no money device yet.' },
}

/** 🧾 Pay a bill · 📤 Send to a wallet · 💵 Cash out — each goes on the cart; the customer pays (or is paid) on the normal Pay. */
export function MoneyTab({ mode, till, data, sym }: { mode: MoneyMode; till: TillApi; data: TillData; sym: string }) {
  const w = WORDS[mode]
  const floats = data.floats.filter((f) => f.money)
  const items = [...floats.map(floatItem), ...(mode === 'deposit' ? [] : data.banks.map((b) => ({ key: `b${b.id}`, label: `🏦 ${b.name}`, balance: b.balance })))]
  const [acct, setAcct] = useState('')
  const key = items.some((i) => i.key === acct) ? acct : (items[0]?.key ?? '')
  const dev = floats.find((f) => `d${f.id}` === key)
  const bank = data.banks.find((b) => `b${b.id}` === key)
  const carrier = data.carriers.find((c) => c.id === dev?.carrierId)
  const [amount, setAmount] = useState('')
  const [charge, setCharge] = useState<string | null>(null) // null = the carrier's default
  const [billType, setBillType] = useState('')
  const [ref, setRef] = useState('')
  const [number, setNumber] = useState('')
  if (!items.length) return <p className="p-6 text-center text-faint">{w.empty}</p>
  const a = num(amount)
  const ch = charge ?? chargeFor(carrier, mode)
  const below = mode !== 'cashout' && goesBelow(dev, inCart(till.lines, dev?.id ?? 0) + a)

  function add() {
    if (!(a > 0)) return void toast.error(`Enter the ${w.amount.toLowerCase()}`)
    const account = dev ? { device: dev } : { bank: bank! }
    if (mode === 'billpay') addBill(till, data, account, a, billType, ref, num(ch))
    else if (mode === 'deposit') addDeposit(till, data, dev!, a, number, num(ch))
    else addCashout(till, data, account, a, num(ch))
    setAmount('')
    setRef('')
    setNumber('')
    setCharge(null)
  }
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1.5">
        <span className="text-[13px] font-semibold text-muted-foreground">{w.account}</span>
        <Pick label={w.account} items={items} value={key} onChange={setAcct} sym={sym} />
      </div>
      {mode === 'billpay' && (
        <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
          <label className={LABEL}>
            Bill
            <input aria-label="Bill" list="recharge-bill-types" autoComplete="off" placeholder="Electricity, Water, Phone…" value={billType}
              onChange={(e) => setBillType(e.target.value)} className={FIELD} />
            <datalist id="recharge-bill-types">{data.billTypes.map((t) => <option key={t} value={t} />)}</datalist>
          </label>
          <label className={LABEL}>
            Account / reference number
            <input aria-label="Account number" autoComplete="off" value={ref} onChange={(e) => setRef(e.target.value)} className={cn(FIELD, 'tabular-nums')} />
          </label>
        </div>
      )}
      {mode === 'deposit' && (
        <label className={LABEL}>
          Customer's wallet number
          <input aria-label="Customer's wallet number" inputMode="tel" autoComplete="off" placeholder="07X XXX XXXX" value={number}
            onChange={(e) => setNumber(e.target.value.replace(/[^\d+ ]/g, ''))} className={cn(FIELD, 'tabular-nums')} />
        </label>
      )}
      <div className="grid gap-3 sm:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
        <Amount label={w.amount} value={amount} onChange={setAmount} big />
        <Amount label="Service charge" value={ch} onChange={setCharge} placeholder={money(sym, 0)} />
      </div>
      <BelowZero show={below} />
      <div className="flex items-center gap-3">
        <p className="mr-auto text-[12.5px] text-muted-foreground">
          {mode === 'cashout' ? 'Goes on the bill as money given; Pay hands it out (netted with anything they buy).' : 'Goes on the bill; they pay on the normal Pay — cash, card, split or on account.'}
        </p>
        <Button size="lg" onClick={add}>{w.add}{a > 0 && ` ${money(sym, a + num(ch))}`}</Button>
      </div>
    </div>
  )
}
