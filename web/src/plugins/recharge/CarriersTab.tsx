import { useQuery } from '@tanstack/react-query'
import { Pencil, Plus } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { EmptyState, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { supplierOptionsQuery } from '@/features/admin/products/queries'
import { configQuery as tillConfigQuery, errMsg } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { carrierActive, carrierSupplier, createCarrier, deviceActive, renameCarrier, reviewPackage, setRates, setupQuery } from './api'
import { DeviceSheet, PackageSheet } from './CarrierSheets'
import type { Carrier, Device, Kind, Package, Rate } from './types'

const KINDS: [Kind, string][] = [['reload', 'Reload'], ['billpay', 'Bill payment'], ['deposit', 'Wallet deposit'], ['cashout', 'Cash out'], ['wallet_in', 'Paid to the shop by wallet']]
const amt = (v: string) => v.replace(/[^\d.]/g, '')

async function run(p: Promise<unknown>, ok?: string) {
  try {
    await p
    if (ok) toast.success(ok)
    return true
  } catch (e) {
    toast.error(errMsg(e))
    return false
  }
}

type Sheet = { kind: 'carrier'; c?: Carrier } | { kind: 'rates'; c: Carrier } | { kind: 'device'; c: Carrier; d?: Device } | { kind: 'package'; c: Carrier; p?: Package } | null

/** Carriers: each with its commission rates, devices and packages (till-added ones flagged until checked). */
export function CarriersTab() {
  const sym = useQuery(tillConfigQuery).data?.symbol ?? 'Rs.'
  const setup = useQuery(setupQuery).data
  const suppliers = useQuery(supplierOptionsQuery).data ?? []
  const [sheet, setSheet] = useState<Sheet>(null)
  if (!setup) return null
  return (
    <div className="grid gap-3">
      <div><Button onClick={() => setSheet({ kind: 'carrier' })}><Plus className="size-4" /> Add carrier</Button></div>
      {setup.carriers.length === 0 && <div className={SURFACE}><EmptyState title="No carriers yet">Add Dialog, Mobitel, Hutch… then each phone or SIM that holds float.</EmptyState></div>}
      {setup.carriers.map((c) => (
        <section key={c.id} aria-label={c.name} className={cn(SURFACE, 'grid gap-3 p-4', !c.isActive && 'opacity-60')}>
          <header className="flex flex-wrap items-center gap-2">
            <h2 className="mr-auto font-display text-lg font-bold">📶 {c.name}{!c.isActive && <small className="ml-2 font-normal text-muted-foreground">turned off</small>}</h2>
            <Button size="sm" variant="ghost" onClick={() => setSheet({ kind: 'carrier', c })}><Pencil className="size-3.5" /> Rename</Button>
            <Button size="sm" variant="ghost" onClick={() => void run(carrierActive(c.id, !c.isActive))}>{c.isActive ? 'Turn off' : 'Turn on'}</Button>
          </header>
          <label className="flex flex-wrap items-center gap-2 text-[13px] text-muted-foreground">
            Float usually bought from
            <select aria-label={`${c.name} float dealer`} value={c.supplierId} onChange={(e) => void run(carrierSupplier(c.id, Number(e.target.value)), 'Dealer saved')}
              className={cn(INPUT, 'h-8 w-auto min-w-0 max-w-full appearance-auto py-0')}>
              <option value={0}>No usual dealer</option>
              {suppliers.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
            </select>
          </label>
          <Rates c={c} onEdit={() => setSheet({ kind: 'rates', c })} />
          <List title="Devices" onAdd={() => setSheet({ kind: 'device', c })} add="Add device">
            {setup.devices.filter((d) => d.carrierId === c.id).map((d) => (
              <li key={d.id} className={cn('flex flex-wrap items-center gap-x-3 gap-y-1 rounded-[var(--radius-sm)] bg-muted px-3 py-2', !d.isActive && 'opacity-60')}>
                <span className="w-full min-w-0 sm:w-auto sm:flex-1"><b className="text-[14px]">{d.label}</b> <small className="text-muted-foreground">{[d.number, d.reload && 'reloads', d.money && 'money', !d.isActive && 'retired'].filter(Boolean).join(' · ')}</small></span>
                {num(d.target) > 0 && <small className="mr-auto whitespace-nowrap tabular-nums text-muted-foreground sm:mr-0">target {money(sym, d.target)}</small>}
                <Button size="sm" variant="ghost" onClick={() => setSheet({ kind: 'device', c, d })}>Edit</Button>
                <Button size="sm" variant="ghost" onClick={() => void run(deviceActive(d.id, !d.isActive))}>{d.isActive ? 'Retire' : 'Bring back'}</Button>
              </li>
            ))}
          </List>
          <List title="Packages" onAdd={() => setSheet({ kind: 'package', c })} add="Add package">
            {setup.packages.filter((p) => p.carrierId === c.id).map((p) => (
              <li key={p.id} className={cn('flex flex-wrap items-center gap-x-3 gap-y-1 rounded-[var(--radius-sm)] bg-muted px-3 py-2', !p.isActive && 'opacity-60')}>
                <span className="w-full min-w-0 sm:w-auto sm:flex-1">
                  <b className="text-[14px]">{p.name}</b>{' '}
                  <small className="tabular-nums text-muted-foreground">{money(sym, p.price)} · uses {money(sym, p.floatCost)} · earns {money(sym, num(p.price) - num(p.floatCost))}</small>
                  {p.addedBy && <span className="ml-2 rounded-md bg-[color-mix(in_srgb,var(--warning)_18%,transparent)] px-1.5 text-[11px] font-bold text-warning">new — added at the till by {p.addedBy}</span>}
                </span>
                {p.addedBy && <Button size="sm" onClick={() => void run(reviewPackage(p.id), `${p.name} checked`)}>Looks right</Button>}
                <Button size="sm" variant="ghost" onClick={() => setSheet({ kind: 'package', c, p })}>Edit</Button>
              </li>
            ))}
          </List>
        </section>
      ))}
      {sheet?.kind === 'carrier' && <CarrierSheet c={sheet.c} onClose={() => setSheet(null)} />}
      {sheet?.kind === 'rates' && <RatesSheet c={sheet.c} onClose={() => setSheet(null)} />}
      {sheet?.kind === 'device' && <DeviceSheet c={sheet.c} d={sheet.d} onClose={() => setSheet(null)} />}
      {sheet?.kind === 'package' && <PackageSheet c={sheet.c} p={sheet.p} sym={sym} onClose={() => setSheet(null)} />}
    </div>
  )
}

function List({ title, add, onAdd, children }: { title: string; add: string; onAdd: () => void; children: React.ReactNode[] }) {
  return (
    <div className="grid gap-1.5">
      <div className="flex items-center justify-between">
        <h3 className="text-[13px] font-bold text-muted-foreground">{title}</h3>
        <Button size="sm" variant="secondary" onClick={onAdd}><Plus className="size-3.5" /> {add}</Button>
      </div>
      {children.length ? <ul className="grid gap-1.5">{children}</ul> : <p className="text-[13px] text-faint">None yet.</p>}
    </div>
  )
}

const rateText = (r: Rate) => [num(r.percent) > 0 && `${num(r.percent)}%`, num(r.fixed) > 0 && `+ ${num(r.fixed)}`].filter(Boolean).join(' ') || '—'

function Rates({ c, onEdit }: { c: Carrier; onEdit: () => void }) {
  return (
    <div className="grid gap-1.5">
      <div className="flex items-center justify-between">
        <h3 className="text-[13px] font-bold text-muted-foreground">Commission the carrier pays</h3>
        <Button size="sm" variant="secondary" onClick={onEdit}>Edit rates</Button>
      </div>
      <dl className="grid grid-cols-[repeat(auto-fill,minmax(150px,1fr))] gap-1.5">
        {KINDS.map(([k, label]) => {
          const r = c.rates.find((x) => x.kind === k)!
          return (
            <div key={k} className="rounded-[var(--radius-sm)] bg-muted px-3 py-2">
              <dt className="text-[12px] text-muted-foreground">{label}</dt>
              <dd className="font-semibold tabular-nums">{rateText(r)}{num(r.charge) > 0 && <small className="font-normal text-muted-foreground"> · charge {num(r.charge)}</small>}</dd>
            </div>
          )
        })}
      </dl>
    </div>
  )
}

function CarrierSheet({ c, onClose }: { c?: Carrier; onClose: () => void }) {
  const [name, setName] = useState(c?.name ?? '')
  const [busy, setBusy] = useState(false)
  async function save() {
    setBusy(true)
    if (await run(c ? renameCarrier(c.id, name) : createCarrier(name))) onClose()
    setBusy(false)
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={c ? `Rename ${c.name}` : 'Add a carrier'} submit={c ? 'Rename' : 'Add'} busy={busy} onSubmit={() => void save()}>
      <Field label="Name" hint={c ? 'A rebrand keeps its devices, packages and history.' : 'e.g. Dialog, Mobitel, Hutch'}>
        <input autoFocus aria-label="Carrier name" value={name} onChange={(e) => setName(e.target.value)} className={INPUT} />
      </Field>
    </FormSheet>
  )
}

/** Rates: a percent and/or a fixed amount per move, and the service charge the till offers. */
function RatesSheet({ c, onClose }: { c: Carrier; onClose: () => void }) {
  const [rs, setRs] = useState<Rate[]>(c.rates)
  const [busy, setBusy] = useState(false)
  const set = (k: Kind, f: 'percent' | 'fixed' | 'charge', v: string) => setRs((x) => x.map((r) => (r.kind === k ? { ...r, [f]: amt(v) || '0' } : r)))
  async function save() {
    setBusy(true)
    if (await run(setRates(c.id, rs), 'Rates saved')) onClose()
    setBusy(false)
  }
  return (
    <FormSheet wide open onOpenChange={(o) => !o && onClose()} title={`${c.name} — commission`} submit="Save rates" busy={busy} onSubmit={() => void save()}>
      <p className="text-sm text-muted-foreground">What {c.name} adds to the float on each move. It's booked as the move happens; counts catch anything else (promos).</p>
      {KINDS.map(([k, label]) => {
        const r = rs.find((x) => x.kind === k)!
        return (
          <fieldset key={k} className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)] gap-2">
            <legend className="mb-1 text-sm font-bold">{label}</legend>
            <Field label="%"><input aria-label={`${label} percent`} inputMode="decimal" value={num(r.percent) ? r.percent : ''} placeholder="0" onChange={(e) => set(k, 'percent', e.target.value)} className={cn(INPUT, 'tabular-nums')} /></Field>
            <Field label="+ fixed"><input aria-label={`${label} fixed`} inputMode="decimal" value={num(r.fixed) ? r.fixed : ''} placeholder="0" onChange={(e) => set(k, 'fixed', e.target.value)} className={cn(INPUT, 'tabular-nums')} /></Field>
            {k === 'reload' || k === 'wallet_in' ? <span /> : (
              <Field label="Service charge"><input aria-label={`${label} service charge`} inputMode="decimal" value={num(r.charge) ? r.charge : ''} placeholder="0" onChange={(e) => set(k, 'charge', e.target.value)} className={cn(INPUT, 'tabular-nums')} /></Field>
            )}
          </fieldset>
        )
      })}
      <p className="text-[13px] text-muted-foreground">Packages earn their price less the float they use — set on each package.</p>
    </FormSheet>
  )
}
