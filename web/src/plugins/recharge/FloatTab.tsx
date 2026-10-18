import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { configQuery, errMsg } from '@/features/till/api'
import { slipPath } from '@/features/till/print'
import { slipOut } from '@/features/till/SlipAsk'
import { FIELD } from '@/features/till/ui'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { counterRefill, sourcesQuery } from './api'
import { Amount } from './bits'
import { LABEL } from './words'
import type { TillData } from './types'

const ago = (iso: string | null) => (iso ? new Date(iso).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' }) : 'never')

/** 💰 Float: every device's float now (red below zero), and buying float from the carrier. */
export function FloatTab({ data, sym }: { data: TillData; sym: string }) {
  const cfg = useSuspenseQuery(configQuery).data
  return (
    <div className="flex flex-col gap-4">
      <ul aria-label="Floats" className="grid grid-cols-[repeat(auto-fill,minmax(min(100%,300px),1fr))] gap-1.5">
        {data.floats.map((f) => (
          <li key={f.id} className="flex items-center justify-between gap-3 rounded-[var(--radius-sm)] bg-muted px-3 py-2">
            <span className="min-w-0">
              <b className="block truncate text-[14px]">{f.carrier} · {f.label}</b>
              <small className="text-muted-foreground">counted {ago(f.lastCount)}</small>
            </span>
            <b className={cn('shrink-0 whitespace-nowrap font-display text-lg tabular-nums', num(f.balance) < 0 && 'text-danger')}>{money(sym, f.balance)}</b>
          </li>
        ))}
      </ul>
      {cfg.canHandleSuppliers && data.floats.length > 0 && <BuyFloat data={data} sym={sym} />}
    </div>
  )
}

function BuyFloat({ data, sym }: { data: TillData; sym: string }) {
  const sources = useQuery(sourcesQuery).data ?? []
  const [device, setDevice] = useState(String(data.floats[0].id))
  const [amount, setAmount] = useState('')
  const [bonus, setBonus] = useState('')
  const [source, setSource] = useState('')
  const [reference, setReference] = useState('')
  const [busy, setBusy] = useState(false)
  const src = source || sources[0]?.value || ''
  async function buy() {
    if (!(num(amount) > 0)) return void toast.error('Enter what you paid for the float')
    setBusy(true)
    try {
      const r = await counterRefill({ deviceId: Number(device), amount, bonus: bonus || '0', source: src, reference })
      void slipOut(slipPath.money(r.receiptId), `Float bought · ${money(sym, amount)}`) // the server already popped the drawer
      setAmount('')
      setBonus('')
      setReference('')
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <section aria-label="Buy float" className="grid gap-3 rounded-[var(--radius-sm)] border border-border p-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
      <h3 className="text-[14px] font-bold sm:col-span-2">Buy float from the carrier</h3>
      <label className={LABEL}>
        Device
        <select aria-label="Device" value={device} onChange={(e) => setDevice(e.target.value)} className={cn(FIELD, 'appearance-auto')}>
          {data.floats.map((f) => <option key={f.id} value={f.id}>{f.carrier} · {f.label}</option>)}
        </select>
      </label>
      <label className={LABEL}>
        Paid from
        <select aria-label="Paid from" value={src} onChange={(e) => setSource(e.target.value)} className={cn(FIELD, 'appearance-auto')}>
          {sources.map((s) => <option key={s.value} value={s.value}>{s.label}{s.value.startsWith('locker:') ? ` (${money(sym, s.balance)})` : ''}</option>)}
        </select>
      </label>
      <Amount label="Amount paid" value={amount} onChange={setAmount} />
      <Amount label="Bonus the carrier added (optional)" value={bonus} onChange={setBonus} placeholder={money(sym, 0)} />
      <label className={cn(LABEL, 'sm:col-span-2')}>
        Reference (optional)
        <input aria-label="Reference" autoComplete="off" value={reference} onChange={(e) => setReference(e.target.value)} className={FIELD} />
      </label>
      <div className="flex items-center gap-3 sm:col-span-2">
        <p className="mr-auto text-[12px] text-muted-foreground">Cash turned into float — not an expense.</p>
        <Button disabled={busy || !src} onClick={() => void buy()}>Buy {num(amount) > 0 ? money(sym, num(amount) + num(bonus)) : 'float'}</Button>
      </div>
    </section>
  )
}
