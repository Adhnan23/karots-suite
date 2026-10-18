import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { queryClient } from '@/api/query-client'
import { supplierOptionsQuery } from '@/features/admin/products/queries'
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

/** Buy float from a dealer (a normal supplier): a supplier bill, paid now from a cash
 * place or a bank (never a card account) or on credit. On the device at once — cash
 * turned into float, not an expense. */
export function BuySheet({ floats, sym, onClose }: { floats: Float[]; sym: string; onClose: () => void }) {
  const sources = (useQuery(moneySourcesQuery).data ?? []).filter((s) => s.kind !== 'card')
  const suppliers = useQuery(supplierOptionsQuery).data ?? []
  const [device, setDevice] = useState(String(floats[0]?.id ?? ''))
  const [supplier, setSupplier] = useState(0)
  const [credit, setCredit] = useState(false)
  const [amount, setAmount] = useState('')
  const [bonus, setBonus] = useState('')
  const [source, setSource] = useState('')
  const [reference, setReference] = useState('')
  const [busy, setBusy] = useState(false)
  const src = source || sources[0]?.value || ''
  const sup = supplier || floats.find((f) => String(f.id) === device)?.supplierId || 0 // the carrier's usual dealer
  async function save() {
    if (!(num(amount) > 0)) return void toast.error('Enter how much float you bought')
    if (!sup) return void toast.error('Pick who sold the float')
    setBusy(true)
    try {
      const r = await adminRefill({ deviceId: Number(device), supplierId: sup, amount, bonus: bonus || '0', credit, source: src, reference })
      if (r.receiptId) void slipOut(slipPath.money(r.receiptId), `Float bought · ${money(sym, amount)}`)
      else toast.success(credit ? 'Float bought on credit' : 'Float bought — their advance covered it')
      void queryClient.invalidateQueries({ queryKey: ['admin'] })
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title="Buy float" submit={`${credit ? 'Buy on credit' : 'Buy'} ${num(amount) > 0 ? money(sym, num(amount) + num(bonus)) : 'float'}`} busy={busy || (!credit && !src)} onSubmit={() => void save()}>
      <Field label="Device">
        <select aria-label="Device" value={device} onChange={(e) => { setDevice(e.target.value); setSupplier(0) }} className={cn(INPUT, 'appearance-auto')}>
          {floats.map((f) => <option key={f.id} value={f.id}>{f.carrier} · {f.label} ({money(sym, f.balance)})</option>)}
        </select>
      </Field>
      <Field label="Bought from" hint="A normal supplier — the carrier's usual dealer is set on Carriers">
        <select aria-label="Bought from" value={sup} onChange={(e) => setSupplier(Number(e.target.value))} className={cn(INPUT, 'appearance-auto')}>
          <option value={0}>Pick a supplier…</option>
          {suppliers.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
        </select>
      </Field>
      <Field label="Paying">
        <select aria-label="Paying" value={credit ? 'credit' : 'now'} onChange={(e) => setCredit(e.target.value === 'credit')} className={cn(INPUT, 'appearance-auto')}>
          <option value="now">Paid now</option>
          <option value="credit">On credit — added to what we owe them</option>
        </select>
      </Field>
      {!credit && (
        <Field label="Paid from">
          <select aria-label="Paid from" value={src} onChange={(e) => setSource(e.target.value)} className={cn(INPUT, 'appearance-auto')}>
            {sources.map((s) => <option key={s.value} value={s.value}>{s.label}{s.kind !== 'till' ? ` (${money(sym, s.balance)})` : ''}</option>)}
          </select>
        </Field>
      )}
      <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)] gap-3">
        <Field label="Float bought"><input aria-label="Float bought" inputMode="decimal" value={amount} onChange={(e) => setAmount(amt(e.target.value))} className={cn(INPUT, 'tabular-nums')} /></Field>
        <Field label="Their bonus" hint="Extra float they added, if any"><input aria-label="Their bonus" inputMode="decimal" value={bonus} onChange={(e) => setBonus(amt(e.target.value))} className={cn(INPUT, 'tabular-nums')} /></Field>
      </div>
      <Field label="Their bill no."><input aria-label="Bill number" value={reference} onChange={(e) => setReference(e.target.value)} placeholder="Optional" className={INPUT} /></Field>
    </FormSheet>
  )
}
