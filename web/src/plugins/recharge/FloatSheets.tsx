import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { moneySourcesQuery } from '@/features/purchasing/MoneySource'
import { errMsg } from '@/features/till/api'
import { slipPath } from '@/features/till/print'
import { slipOut } from '@/features/till/SlipAsk'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { adminCounts, adminRefill } from './api'
import type { Float } from './types'

const amt = (v: string) => v.replace(/[^\d.]/g, '')

/** Count now: what each phone shows (blank = skip); the differences are booked. */
export function CountSheet({ floats, sym, onClose }: { floats: Float[]; sym: string; onClose: () => void }) {
  const [v, setV] = useState<Record<number, string>>({})
  const [busy, setBusy] = useState(false)
  async function save() {
    const counts = floats.filter((f) => (v[f.id] ?? '') !== '').map((f) => ({ deviceId: f.id, counted: v[f.id] }))
    if (!counts.length) return void toast.error('Type what at least one phone shows')
    setBusy(true)
    try {
      const res = await adminCounts(counts)
      const sum = res.reduce((s, r) => s + num(r.diff), 0)
      toast.success(Math.abs(sum) < 0.005 ? 'Counted — every float matches' : `Counted — ${sum > 0 ? '+' : ''}${money(sym, sum)} booked`)
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title="Count the floats" submit="Save the count" busy={busy} onSubmit={() => void save()}>
      <p className="text-sm text-muted-foreground">Type what each phone shows now. A difference is booked as extra (a promo, a rate change) or short.</p>
      {floats.map((f) => {
        const d = (v[f.id] ?? '') === '' ? 0 : num(v[f.id]) - num(f.balance)
        return (
          <Field key={f.id} label={`${f.carrier} · ${f.label}`}
            hint={<span className="tabular-nums">expected {money(sym, f.balance)}{Math.abs(d) > 0.005 && <b className={d > 0 ? 'text-success' : 'text-danger'}> · {d > 0 ? '+' : ''}{money(sym, d)}</b>}</span>}>
            <input aria-label={`Float on ${f.carrier} ${f.label}`} inputMode="decimal" placeholder={num(f.balance).toFixed(2)} value={v[f.id] ?? ''}
              onChange={(e) => setV((x) => ({ ...x, [f.id]: amt(e.target.value) }))} className={cn(INPUT, 'tabular-nums')} />
          </Field>
        )
      })}
    </FormSheet>
  )
}

/** Buy float: paid from a cash place or a bank (never a card account), on the device
 * at once — cash turned into float, not an expense. */
export function BuySheet({ floats, sym, onClose }: { floats: Float[]; sym: string; onClose: () => void }) {
  const sources = (useQuery(moneySourcesQuery).data ?? []).filter((s) => s.kind !== 'card')
  const [device, setDevice] = useState(String(floats[0]?.id ?? ''))
  const [amount, setAmount] = useState('')
  const [bonus, setBonus] = useState('')
  const [source, setSource] = useState('')
  const [reference, setReference] = useState('')
  const [busy, setBusy] = useState(false)
  const src = source || sources[0]?.value || ''
  async function save() {
    if (!(num(amount) > 0)) return void toast.error('Enter what you paid')
    setBusy(true)
    try {
      const r = await adminRefill({ deviceId: Number(device), amount, bonus: bonus || '0', source: src, reference })
      void slipOut(slipPath.money(r.receiptId), `Float bought · ${money(sym, amount)}`)
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title="Buy float" submit={`Buy ${num(amount) > 0 ? money(sym, num(amount) + num(bonus)) : 'float'}`} busy={busy || !src} onSubmit={() => void save()}>
      <Field label="Device">
        <select aria-label="Device" value={device} onChange={(e) => setDevice(e.target.value)} className={cn(INPUT, 'appearance-auto')}>
          {floats.map((f) => <option key={f.id} value={f.id}>{f.carrier} · {f.label} ({money(sym, f.balance)})</option>)}
        </select>
      </Field>
      <Field label="Paid from">
        <select aria-label="Paid from" value={src} onChange={(e) => setSource(e.target.value)} className={cn(INPUT, 'appearance-auto')}>
          {sources.map((s) => <option key={s.value} value={s.value}>{s.label}{s.kind !== 'till' ? ` (${money(sym, s.balance)})` : ''}</option>)}
        </select>
      </Field>
      <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)] gap-3">
        <Field label="Amount paid"><input aria-label="Amount paid" inputMode="decimal" value={amount} onChange={(e) => setAmount(amt(e.target.value))} className={cn(INPUT, 'tabular-nums')} /></Field>
        <Field label="Carrier's bonus" hint="Extra float they added, if any"><input aria-label="Carrier's bonus" inputMode="decimal" value={bonus} onChange={(e) => setBonus(amt(e.target.value))} className={cn(INPUT, 'tabular-nums')} /></Field>
      </div>
      <Field label="Reference"><input aria-label="Reference" value={reference} onChange={(e) => setReference(e.target.value)} placeholder="Optional" className={INPUT} /></Field>
    </FormSheet>
  )
}
