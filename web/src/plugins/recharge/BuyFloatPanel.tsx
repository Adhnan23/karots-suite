import { useQuery } from '@tanstack/react-query'
import { X } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { configQuery, errMsg } from '@/features/till/api'
import { slipPath } from '@/features/till/print'
import { slipOut } from '@/features/till/SlipAsk'
import { FIELD } from '@/features/till/ui'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { counterRefill, sourcesQuery, tillDataQuery } from './api'
import { Amount } from './bits'
import { LABEL } from './words'

/** Counter → Suppliers → 📲 Buy reload float: the carrier supplies the float, paid from
 * this drawer or a cashier locker; on the device at once (not an expense). */
export function BuyFloatPanel({ onClose }: { onClose: () => void }) {
  const sym = useQuery(configQuery).data?.symbol ?? ''
  const floats = useQuery(tillDataQuery).data?.floats ?? []
  const sources = useQuery(sourcesQuery).data ?? []
  const [device, setDevice] = useState('')
  const [amount, setAmount] = useState('')
  const [bonus, setBonus] = useState('')
  const [source, setSource] = useState('')
  const [reference, setReference] = useState('')
  const [busy, setBusy] = useState(false)
  const dev = device || String(floats[0]?.id ?? '')
  const src = source || sources[0]?.value || ''
  const picked = floats.find((f) => String(f.id) === dev)
  async function buy() {
    if (!(num(amount) > 0)) return void toast.error('Enter what you paid for the float')
    setBusy(true)
    try {
      const r = await counterRefill({ deviceId: Number(dev), amount, bonus: bonus || '0', source: src, reference })
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
    <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-auto p-5">
      <header className="flex items-start justify-between gap-3">
        <div>
          <h2 className="font-display text-xl font-bold">📲 Buy reload float</h2>
          <p className="text-[13px] text-muted-foreground">Paid to the carrier; it lands on the device straight away. Cash turned into float — not an expense.</p>
        </div>
        <Button size="icon" variant="ghost" aria-label="Close" onClick={onClose}><X className="size-4" /></Button>
      </header>
      {floats.length === 0 ? <p className="m-auto text-faint">No reload devices yet — an admin adds them in Apps → Reload &amp; Bills.</p> : (
        <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
          <label className={LABEL}>
            Device
            <select aria-label="Device" value={dev} onChange={(e) => setDevice(e.target.value)} className={cn(FIELD, 'appearance-auto')}>
              {floats.map((f) => <option key={f.id} value={f.id}>{f.carrier} · {f.label}</option>)}
            </select>
            {picked && <small className="font-normal tabular-nums">float now {money(sym, picked.balance)}</small>}
          </label>
          <label className={LABEL}>
            Paid from
            <select aria-label="Paid from" value={src} onChange={(e) => setSource(e.target.value)} className={cn(FIELD, 'appearance-auto')}>
              {sources.map((s) => <option key={s.value} value={s.value}>{s.label}{s.value.startsWith('locker:') ? ` (${money(sym, s.balance)})` : ''}</option>)}
            </select>
          </label>
          <Amount label="Amount paid" value={amount} onChange={setAmount} big />
          <Amount label="Bonus the carrier added (optional)" value={bonus} onChange={setBonus} placeholder={money(sym, 0)} />
          <label className={cn(LABEL, 'sm:col-span-2')}>
            Reference (optional)
            <input aria-label="Reference" autoComplete="off" value={reference} onChange={(e) => setReference(e.target.value)} className={FIELD} />
          </label>
          <div className="flex justify-end sm:col-span-2">
            <Button size="lg" disabled={busy || !src} onClick={() => void buy()}>Buy {num(amount) > 0 ? money(sym, num(amount) + num(bonus)) : 'float'}</Button>
          </div>
        </div>
      )}
    </div>
  )
}
