import { useState } from 'react'
import { toast } from 'sonner'
import { Field, INPUT, Toggle } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { errMsg } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { createDevice, createPackage, updateDevice, updatePackage } from './api'
import type { Carrier, Device, Package } from './types'

const amt = (v: string) => v.replace(/[^\d.]/g, '')

/** A phone / SIM holding float: for reloads, money or both; the float already on it when added. */
export function DeviceSheet({ c, d, onClose }: { c: Carrier; d?: Device; onClose: () => void }) {
  const [label, setLabel] = useState(d?.label ?? '')
  const [number, setNumber] = useState(d?.number ?? '')
  const [reload, setReload] = useState(d?.reload ?? true)
  const [moneyUse, setMoney] = useState(d?.money ?? true)
  const [target, setTarget] = useState(d && num(d.target) ? d.target : '')
  const [opening, setOpening] = useState('')
  const [busy, setBusy] = useState(false)
  async function save() {
    setBusy(true)
    try {
      const body = { carrierId: c.id, label, number, reload, money: moneyUse, target: target || '0', opening: opening || '0' }
      await (d ? updateDevice(d.id, body) : createDevice(body))
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={d ? `Edit ${d.label}` : `Add a ${c.name} device`} submit={d ? 'Save' : 'Add'} busy={busy} onSubmit={() => void save()}>
      <Field label="Name"><input autoFocus aria-label="Device name" placeholder="Shop phone, Reload SIM…" value={label} onChange={(e) => setLabel(e.target.value)} className={INPUT} /></Field>
      <Field label="Number"><input aria-label="Device number" inputMode="tel" value={number} onChange={(e) => setNumber(e.target.value)} className={INPUT} /></Field>
      <Toggle label="Reloads and packages" checked={reload} onChange={setReload} />
      <Toggle label="Money (bills, deposits, cash out, wallet payments)" hint="Two floats on one SIM? Add it twice, one for each." checked={moneyUse} onChange={setMoney} />
      <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)] gap-3">
        <Field label="Target float" hint="Top-up report shows the gap"><input aria-label="Target float" inputMode="decimal" value={target} onChange={(e) => setTarget(amt(e.target.value))} className={cn(INPUT, 'tabular-nums')} /></Field>
        {!d && <Field label="Float on it now" hint="Like opening stock — no expense"><input aria-label="Float on it now" inputMode="decimal" value={opening} onChange={(e) => setOpening(amt(e.target.value))} className={cn(INPUT, 'tabular-nums')} /></Field>}
      </div>
    </FormSheet>
  )
}

/** A package: price to the customer and the float it uses (or the commission the app shows). */
export function PackageSheet({ c, p, sym, onClose }: { c: Carrier; p?: Package; sym: string; onClose: () => void }) {
  const [name, setName] = useState(p?.name ?? '')
  const [price, setPrice] = useState(p?.price ?? '')
  const [commission, setCommission] = useState(p ? String(num(p.price) - num(p.floatCost)) : '')
  const [sort, setSort] = useState(String(p?.sort ?? 0))
  const [active, setActive] = useState(p?.isActive ?? true)
  const [busy, setBusy] = useState(false)
  const floatCost = num(price) - num(commission)
  async function save() {
    setBusy(true)
    try {
      const body = { carrierId: c.id, name, price: price || '0', floatCost: String(floatCost), sort: Number(sort) || 0, isActive: active }
      await (p ? updatePackage(p.id, body) : createPackage(body))
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={p ? `Edit ${p.name}` : `Add a ${c.name} package`} submit={p ? 'Save' : 'Add'} busy={busy} onSubmit={() => void save()}>
      <Field label="Name"><input autoFocus aria-label="Package name" placeholder="Data 100, Night 50…" value={name} onChange={(e) => setName(e.target.value)} className={INPUT} /></Field>
      <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)] gap-3">
        <Field label="Price"><input aria-label="Package price" inputMode="decimal" value={price} onChange={(e) => setPrice(amt(e.target.value))} className={cn(INPUT, 'tabular-nums')} /></Field>
        <Field label="Commission" hint={num(price) > 0 ? `uses ${money(sym, floatCost)} of float` : 'from the carrier\'s app'}>
          <input aria-label="Package commission" inputMode="decimal" value={commission} onChange={(e) => setCommission(amt(e.target.value))} className={cn(INPUT, 'tabular-nums')} />
        </Field>
      </div>
      <Field label="Order" hint="Lower comes first on the till"><input aria-label="Order" inputMode="numeric" value={sort} onChange={(e) => setSort(e.target.value.replace(/\D/g, ''))} className={cn(INPUT, 'w-24')} /></Field>
      {p && <Toggle label="On sale" checked={active} onChange={setActive} />}
    </FormSheet>
  )
}
