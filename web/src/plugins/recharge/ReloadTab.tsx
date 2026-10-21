import { motion } from 'motion/react'
import { tx } from '@/theme/motion'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'
import { CHIP, FIELD, TILE } from '@/features/till/ui'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import type { TillApi } from '../slots'
import { quickPackage } from './api'
import { Amount, BelowZero, Pick } from './bits'
import { LABEL, QUICK, floatItem } from './words'
import { addPackage, addReload, goesBelow, inCart } from './till'
import type { Package, TillData } from './types'

/** 📶 Reload: carrier → device → amount or a package tile (and + New package mid-rush). */
export function ReloadTab({ till, data, sym }: { till: TillApi; data: TillData; sym: string }) {
  const carriers = data.carriers.filter((c) => data.floats.some((f) => f.reload && f.carrierId === c.id))
  const [cid, setCid] = useState(carriers[0]?.id ?? 0)
  const carrier = carriers.find((c) => c.id === cid) ?? carriers[0]
  const devices = data.floats.filter((f) => f.reload && f.carrierId === carrier?.id)
  const [did, setDid] = useState('')
  const dev = devices.find((f) => `d${f.id}` === did) ?? devices[0]
  const [number, setNumber] = useState('')
  const [amount, setAmount] = useState('')
  const [adding, setAdding] = useState(false)
  if (!carrier || !dev) return <p className="p-6 text-center text-faint">No reload devices yet. Add a carrier and its phone in Apps → Reload &amp; Bills.</p>
  const used = inCart(till.lines, dev.id)
  const a = num(amount)
  const packages = data.packages.filter((p) => p.carrierId === carrier.id)

  const reload = () => {
    if (!(a > 0)) return void toast.error('Enter the reload amount')
    addReload(till, carrier, dev, a, number)
    setAmount('')
    setNumber('')
  }
  const sell = (p: Package) => {
    addPackage(till, carrier, dev, p, number)
    setNumber('')
  }
  return (
    <div className="flex flex-col gap-4">
      {carriers.length > 1 && (
        <div role="group" aria-label="Carrier" className="flex flex-wrap gap-1.5">
          {carriers.map((c) => <button key={c.id} type="button" className={CHIP} aria-pressed={c.id === carrier.id} onClick={() => { setCid(c.id); setDid('') }}>📶 {c.name}</button>)}
        </div>
      )}
      {devices.length > 1 && <Pick label="Device" items={devices.map(floatItem)} value={`d${dev.id}`} onChange={setDid} sym={sym} />}
      <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <label className={LABEL}>
          Customer's number (optional)
          <input aria-label="Customer's number" inputMode="tel" autoComplete="off" placeholder="07X XXX XXXX" value={number}
            onChange={(e) => setNumber(e.target.value.replace(/[^\d+ ]/g, ''))} className={cn(FIELD, 'tabular-nums')} />
        </label>
        <Amount label="Reload amount" value={amount} onChange={setAmount} />
      </div>
      <div className="flex flex-wrap items-center gap-1.5">
        {QUICK.map((q) => <button key={q} type="button" className={CHIP} aria-pressed={a === q} onClick={() => setAmount(String(q))}>{money(sym, q)}</button>)}
        <Button className="ml-auto" onClick={reload}>Add reload{a > 0 && ` ${money(sym, a)}`}</Button>
      </div>
      <BelowZero show={goesBelow(dev, used + a)} />
      <section aria-label="Packages" className="flex flex-col gap-2">
        <h3 className="text-[13px] font-bold text-muted-foreground">{carrier.name} packages</h3>
        <div className="grid grid-cols-[repeat(auto-fill,minmax(140px,1fr))] gap-2">
          {packages.map((p) => (
            <motion.button key={p.id} type="button" onClick={() => sell(p)} whileHover={{ y: -2 }} whileTap={{ scale: 0.97 }} transition={tx('fast')}
              className={cn(TILE, 'min-h-[78px]')}>
              <span className="text-[14px] font-semibold leading-tight">{p.name}</span>
              <b className="font-display text-[15px] tabular-nums">{money(sym, p.price)}</b>
            </motion.button>
          ))}
          <button type="button" onClick={() => setAdding(true)} aria-expanded={adding}
            className={cn(TILE, 'min-h-[78px] items-center justify-center border-dashed text-muted-foreground')}>
            <Plus className="size-5" /><span className="text-[13px] font-semibold">New package</span>
          </button>
        </div>
        {adding && <NewPackage carrierId={carrier.id} sym={sym} onSold={(p) => { sell(p); setAdding(false) }} onCancel={() => setAdding(false)} />}
      </section>
    </div>
  )
}

/** A package the system hasn't seen: name, price and the commission the carrier's app shows — saved and sold in one go. */
function NewPackage({ carrierId, sym, onSold, onCancel }: { carrierId: number; sym: string; onSold: (p: Package) => void; onCancel: () => void }) {
  const [name, setName] = useState('')
  const [price, setPrice] = useState('')
  const [commission, setCommission] = useState('')
  const [busy, setBusy] = useState(false)
  async function save() {
    if (!name.trim() || !(num(price) > 0) || commission === '') return void toast.error('Name, price and the commission from the app')
    setBusy(true)
    try {
      const r = await quickPackage(carrierId, name.trim(), price, commission)
      if (!r.created) toast(`${r.package.name} was already there — sold that one`)
      onSold(r.package)
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <div role="group" aria-label="New package" className="grid gap-3 rounded-[var(--radius-sm)] border border-border bg-muted/50 p-3 sm:grid-cols-[minmax(0,2fr)_minmax(0,1fr)_minmax(0,1fr)]">
      <label className={LABEL}>
        Package name
        <input autoFocus aria-label="Package name" autoComplete="off" placeholder="e.g. Night 50" value={name} onChange={(e) => setName(e.target.value)} className={cn(FIELD, "w-full min-w-0")} />
      </label>
      <Amount label="Package price" value={price} onChange={setPrice} />
      <Amount label="Commission (app)" value={commission} onChange={setCommission} placeholder={money(sym, 0)} />
      <div className="flex items-center gap-2 sm:col-span-3">
        <p className="mr-auto text-[12px] text-muted-foreground">Shown in the carrier's app for this package. An admin checks new packages later.</p>
        <Button variant="secondary" onClick={onCancel}>Cancel</Button>
        <Button disabled={busy} onClick={() => void save()}>Save &amp; sell</Button>
      </div>
    </div>
  )
}
