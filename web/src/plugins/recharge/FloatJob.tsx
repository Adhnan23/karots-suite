import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import type { SupplierView } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Seg } from '@/features/till/bits'
import { configQuery, errMsg } from '@/features/till/api'
import { slipPath } from '@/features/till/print'
import { slipOut } from '@/features/till/SlipAsk'
import { FIELD } from '@/features/till/ui'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { counterRefill, sourcesQuery, tillDataQuery } from './api'
import { Amount, Pick } from './bits'
import { LABEL } from './words'

/** Counter → Suppliers → a dealer → 📲 Float: float bought from this supplier is a normal
 * supplier bill — paid now from this drawer or a cashier locker, or on credit (settled
 * later with Pay). On the device at once; cash turned into float, not an expense. */
export function FloatJob({ supplier }: { supplier: SupplierView }) {
  const sym = useQuery(configQuery).data?.symbol ?? ''
  const qc = useQueryClient()
  // this dealer's carriers first
  const floats = [...(useQuery(tillDataQuery).data?.floats ?? [])].sort((a, b) => Number(b.supplierId === supplier.id) - Number(a.supplierId === supplier.id))
  const sources = useQuery(sourcesQuery).data ?? []
  const [device, setDevice] = useState('')
  const [pay, setPay] = useState<'now' | 'credit'>('now')
  const [amount, setAmount] = useState('')
  const [bonus, setBonus] = useState('')
  const [source, setSource] = useState('')
  const [reference, setReference] = useState('')
  const [busy, setBusy] = useState(false)
  const dev = device || String(floats[0]?.id ?? '')
  const src = source || sources[0]?.value || ''
  const advance = Math.max(0, -num(supplier.owed))
  async function buy() {
    if (!(num(amount) > 0)) return void toast.error('Enter how much float you bought')
    setBusy(true)
    try {
      const r = await counterRefill({ deviceId: Number(dev), supplierId: supplier.id, amount, bonus: bonus || '0', credit: pay === 'credit', source: src, reference })
      if (r.receiptId) void slipOut(slipPath.money(r.receiptId), `Float bought · ${money(sym, amount)}`) // the server already popped the drawer
      else toast.success(pay === 'credit' ? `Float on credit — we owe ${supplier.name} ${money(sym, amount)}` : 'Float bought — their advance covered it')
      void qc.invalidateQueries({ queryKey: ['till', 'supplier', supplier.id] })
      void qc.invalidateQueries({ queryKey: ['till', 'suppliers'] })
      setAmount('')
      setBonus('')
      setReference('')
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  if (floats.length === 0) return <p className="m-auto p-8 text-center text-faint">No reload devices yet — an admin adds them in Apps → Reload &amp; Bills.</p>
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-auto px-5 pb-5">
      <div className={LABEL}>
        Device
        <Pick label="Device" sym={sym} value={dev} onChange={setDevice}
          items={floats.map((f) => ({ key: String(f.id), label: `${f.carrier} · ${f.label}`, balance: f.balance }))} />
      </div>
      <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <Amount label="Float bought" value={amount} onChange={setAmount} big />
        <Amount label="Bonus they added (optional)" value={bonus} onChange={setBonus} placeholder={money(sym, 0)} />
        <div className={LABEL}>
          Paying
          <div><Seg label="Paying" value={pay} onChange={setPay} options={[['now', 'Paid now'], ['credit', 'On credit']]} /></div>
        </div>
        {pay === 'now' ? (
          <label className={LABEL}>
            Paid from
            <select aria-label="Paid from" value={src} onChange={(e) => setSource(e.target.value)} className={cn(FIELD, 'appearance-auto')}>
              {sources.map((s) => <option key={s.value} value={s.value}>{s.label}{s.value.startsWith('locker:') ? ` (${money(sym, s.balance)})` : ''}</option>)}
            </select>
          </label>
        ) : <p className="self-end text-[13px] text-muted-foreground">Added to what we owe {supplier.name} — pay it later with Pay.</p>}
        <label className={cn(LABEL, 'sm:col-span-2')}>
          Their bill no. (optional)
          <input aria-label="Bill number" autoComplete="off" value={reference} onChange={(e) => setReference(e.target.value)} className={FIELD} />
        </label>
        {pay === 'now' && advance > 0 && <p className="text-[13px] text-muted-foreground sm:col-span-2">They hold {money(sym, advance)} of ours — that's used first.</p>}
        <div className="flex justify-end sm:col-span-2">
          <Button size="lg" disabled={busy || (pay === 'now' && !src)} onClick={() => void buy()}>
            {pay === 'credit' ? 'Buy on credit' : 'Buy'} {num(amount) > 0 ? money(sym, num(amount) + num(bonus)) : 'float'}
          </Button>
        </div>
      </div>
    </div>
  )
}
