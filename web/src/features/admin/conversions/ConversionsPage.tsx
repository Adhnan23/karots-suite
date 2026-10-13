import { keepPreviousData, queryOptions, useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { ArrowRight, Download, Plus, Search } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminConversion, AdminProductRow, ConversionRunList } from '@/api/types.gen'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { DataList } from '@/components/admin/DataList'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { EmptyState, PageHead } from '@/components/admin/PageHead'
import { Pager } from '@/components/admin/Pager'
import { Button } from '@/components/ui/button'
import { ProductPicker } from '@/features/admin/products/ProductPicker'
import { Tag } from '@/features/admin/products/ProductsPage'
import { LATEST, rangeQS, type Range } from '@/features/counter/range'
import { RangePicker } from '@/features/counter/RangePicker'
import { Seg } from '@/features/till/bits'
import { configQuery, errMsg } from '@/features/till/api'
import { num, qtyText } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import { yieldNote } from './yield'

const convQuery = (q: string, disabled: boolean) => queryOptions({ queryKey: ['admin', 'conversions', q, disabled], queryFn: () => api<AdminConversion[]>(`/admin/conversions?q=${encodeURIComponent(q)}&disabled=${disabled ? 1 : 0}`) })
const runsQuery = (conv: number, r: Range, page: number) => queryOptions({ queryKey: ['admin', 'conversion-runs', conv, r, page], queryFn: () => api<ConversionRunList>(`/admin/conversions/runs?conversion=${conv}&page=${page}${rangeQS(r)}`) })
const clean = (v: string) => v.replace(/[^\d.]/g, '')
const CHIP = 'rounded-full border border-border bg-card px-3.5 py-2 text-[13px] font-semibold text-muted-foreground aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground'

/** Repack / break bulk into sellable units (karots-pos Product Conversions). */
export function ConversionsPage() {
  const [view, setView] = useState<'recipes' | 'runs'>('recipes')
  return (
    <>
      <PageHead title="Conversions" sub={view === 'recipes' ? 'Break a pack into the units you sell: 1 box of 12 → 12 pieces.' : 'Every pack broken — what went out, what came in, who did it.'}
        actions={<Seg label="View" value={view} onChange={setView} options={[['recipes', 'Conversions'], ['runs', 'Run history']]} />} />
      {view === 'recipes' ? <Recipes /> : <Runs />}
    </>
  )
}

type Editing = { kind: 'new' } | { kind: 'edit'; c: AdminConversion } | { kind: 'run'; c: AdminConversion }

function Recipes() {
  const [q, setQ] = useState('')
  const dq = useDebounced(q.trim(), 250)
  const [disabled, setDisabled] = useState(false)
  const rows = useQuery({ ...convQuery(dq, disabled), placeholderData: keepPreviousData }).data
  const [editing, setEditing] = useState<Editing | null>(null)
  return (
    <>
      <div className="mb-3 grid gap-2 md:flex md:items-center">
        <div className="relative md:w-80">
          <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
          <input aria-label="Search conversions" placeholder="Product name" value={q} onChange={(e) => setQ(e.target.value)} className={cn(INPUT, 'pl-9')} />
        </div>
        <button type="button" aria-pressed={disabled} onClick={() => setDisabled(!disabled)} className={cn(CHIP, 'justify-self-start')}>Show disabled</button>
        <Button className="h-10 gap-1.5 px-4 font-bold md:ml-auto" onClick={() => setEditing({ kind: 'new' })}><Plus size={16} /> New conversion</Button>
      </div>
      <DataList label="Conversions" rows={rows ?? []} rowKey={(c) => c.id} onRow={(c) => setEditing({ kind: c.isActive ? 'run' : 'edit', c })}
        empty={<EmptyState title={rows ? 'No conversions' : 'Loading…'}>{rows && !dq && 'Set one up for anything you buy in packs but sell loose.'}</EmptyState>}
        columns={[
          { key: 'what', header: 'Conversion', cell: (c) => (
            <span className="flex flex-col gap-0.5">
              <span className="flex flex-wrap items-center gap-1.5 font-semibold">{c.fromName} <ArrowRight size={14} className="text-faint" /> {c.toName}{!c.isActive && <Tag tone="muted">disabled</Tag>}</span>
              <small className="text-faint">1 {c.fromUnit} = {qtyText(c.ratio)} {c.toUnit}{c.note && ` · ${c.note}`}</small>
            </span>
          ) },
          { key: 'from', header: 'Packs on hand', num: true, cell: (c) => <b className={cn(num(c.fromStock) <= 0 && 'text-faint')}>{qtyText(c.fromStock)} <small className="font-normal text-faint">{c.fromUnit}</small></b> },
          { key: 'to', header: 'Loose on hand', num: true, cell: (c) => <span>{qtyText(c.toStock)} <small className="text-faint">{c.toUnit}</small></span> },
          { key: 'act', header: '', wide: true, interactive: true, className: 'w-[1%] whitespace-nowrap', cell: (c) => (
            <span className="flex justify-end gap-1">
              {c.isActive && <Button className="h-9 px-3 font-bold" disabled={num(c.fromStock) <= 0} onClick={() => setEditing({ kind: 'run', c })}>Run</Button>}
              <Button variant="ghost" className="h-9 px-3" onClick={() => setEditing({ kind: 'edit', c })}>Edit</Button>
            </span>
          ) },
        ]} />
      {editing?.kind === 'new' && <NewConversion onClose={() => setEditing(null)} />}
      {editing?.kind === 'edit' && <EditConversion key={editing.c.id} c={editing.c} onClose={() => setEditing(null)} />}
      {editing?.kind === 'run' && <RunConversion key={editing.c.id} c={editing.c} onClose={() => setEditing(null)} />}
    </>
  )
}

function useDone(onClose: () => void) {
  const qc = useQueryClient()
  return (msg: string) => { toast.success(msg); void qc.invalidateQueries({ queryKey: ['admin'] }); void qc.invalidateQueries({ queryKey: ['till'] }); onClose() }
}

function NewConversion({ onClose }: { onClose: () => void }) {
  const done = useDone(onClose)
  const [from, setFrom] = useState<AdminProductRow | null>(null)
  const [to, setTo] = useState<AdminProductRow | null>(null)
  const [ratio, setRatio] = useState('')
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const pick = (label: string, p: AdminProductRow | null, set: (p: AdminProductRow) => void, other?: number) => (
    <ProductPicker title={label} exclude={other ? new Set([other]) : undefined} onPick={set} trigger={(open) => (
      <button type="button" aria-label={label} onClick={open} className={cn(INPUT, 'truncate text-left', !p && 'text-faint')}>{p ? `${p.name} · ${p.unitAbbr}` : 'Pick a product'}</button>
    )} />
  )
  async function save() {
    setBusy(true)
    try {
      await api('/admin/conversions', { json: { fromProductId: from?.id ?? 0, toProductId: to?.id ?? 0, ratio, note } })
      done('Conversion added')
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title="New conversion" submit="Add conversion" busy={busy || !from || !to || !(num(ratio) > 0)} onSubmit={save}>
      <Field label="Break this (the pack)">{pick('Pack product', from, setFrom, to?.id)}</Field>
      <Field label="Into this (what you sell)">{pick('Loose product', to, setTo, from?.id)}</Field>
      <Field label={`How many ${to?.unitAbbr ?? 'units'} in one ${from?.unitAbbr ?? 'pack'}`}>
        <input aria-label="Ratio" inputMode="decimal" value={ratio} onChange={(e) => setRatio(clean(e.target.value))} className={cn(INPUT, 'tabular-nums')} placeholder="12" />
      </Field>
      <Field label="Note"><input aria-label="Note" value={note} onChange={(e) => setNote(e.target.value)} className={INPUT} placeholder="Optional" /></Field>
    </FormSheet>
  )
}

function EditConversion({ c, onClose }: { c: AdminConversion; onClose: () => void }) {
  const done = useDone(onClose)
  const [ratio, setRatio] = useState(c.ratio)
  const [note, setNote] = useState(c.note)
  const [busy, setBusy] = useState(false)
  async function save() {
    setBusy(true)
    try {
      await api(`/admin/conversions/${c.id}`, { method: 'PUT', json: { fromProductId: 0, toProductId: 0, ratio, note } })
      done('Conversion saved')
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={`${c.fromName} → ${c.toName}`} submit="Save" busy={busy || !(num(ratio) > 0)} onSubmit={save}
      extra={c.isActive ? (
        <ConfirmButton label="Disable" title="Disable this conversion?" confirm="Disable" body="It leaves the till's Convert tab. Its run history stays."
          onConfirm={async () => { await api(`/admin/conversions/${c.id}`, { method: 'DELETE' }); done('Conversion disabled') }} />
      ) : (
        <Button type="button" variant="outline" className="h-10 px-4" onClick={() => api(`/admin/conversions/${c.id}/activate`, { method: 'POST' }).then(() => done('Conversion enabled'), (e) => toast.error(errMsg(e)))}>Enable</Button>
      )}>
      <Field label={`${c.toUnit} in one ${c.fromUnit}`}>
        <input autoFocus aria-label="Ratio" inputMode="decimal" value={ratio} onChange={(e) => setRatio(clean(e.target.value))} className={cn(INPUT, 'tabular-nums')} />
      </Field>
      <Field label="Note"><input aria-label="Note" value={note} onChange={(e) => setNote(e.target.value)} className={INPUT} placeholder="Optional" /></Field>
    </FormSheet>
  )
}

function RunConversion({ c, onClose }: { c: AdminConversion; onClose: () => void }) {
  const done = useDone(onClose)
  const sym = useSuspenseQuery(configQuery).data.symbol
  const [qty, setQty] = useState('1')
  // null = "what the ratio says"; typed = what was really weighed out.
  const [got, setGot] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const expected = num(qty) * num(c.ratio)
  const makes = got === null ? expected : num(got)
  const note = yieldNote(num(qty), expected, makes, c.toUnit, { sym, perPack: num(c.fromCost) })
  async function save() {
    setBusy(true)
    try {
      await api(`/admin/conversions/${c.id}/run`, { json: { quantity: qty, toQty: note ? String(makes) : undefined } })
      done(`${qtyText(qty)} ${c.fromUnit} → ${qtyText(makes)} ${c.toUnit} ${c.toName}`)
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Break ${c.fromName}`} submit={`Make ${qtyText(makes)} ${c.toUnit}`} busy={busy || !(num(qty) > 0) || num(qty) > num(c.fromStock) || !(makes > 0)} onSubmit={save}>
      <Field label={`How many ${c.fromUnit} to break`} hint={`${qtyText(c.fromStock)} ${c.fromUnit} on hand`}>
        <input autoFocus aria-label="Packs to break" inputMode="decimal" value={qty} onChange={(e) => setQty(clean(e.target.value))} className={cn(INPUT, 'h-14 font-display text-2xl font-bold tabular-nums')} />
      </Field>
      <Field label={`Got (${c.toUnit})`} hint={note ? <span className="font-semibold text-warning">{note}</span> : 'What really came out — leave it if it matched.'}>
        <input aria-label="Got" inputMode="decimal" value={got ?? qtyText(expected)} onChange={(e) => setGot(clean(e.target.value))}
          className={cn(INPUT, 'tabular-nums', note && 'border-warning')} />
      </Field>
      <div className="grid grid-cols-2 gap-2 text-center">
        <div className="rounded-[var(--radius-sm)] bg-[color-mix(in_srgb,var(--danger)_10%,transparent)] p-3"><small className="text-danger">↑ Out</small><b className="block font-display text-xl tabular-nums">{qtyText(qty)} {c.fromUnit}</b><small className="text-faint">{c.fromName}</small></div>
        <div className="rounded-[var(--radius-sm)] bg-[color-mix(in_srgb,var(--success)_10%,transparent)] p-3"><small className="text-success">↓ In</small><b className="block font-display text-xl tabular-nums">{qtyText(makes)} {c.toUnit}</b><small className="text-faint">{c.toName}</small></div>
      </div>
    </FormSheet>
  )
}

function Runs() {
  const convs = useQuery(convQuery('', true)).data ?? []
  const [conv, setConv] = useState(0)
  const [range, setRange] = useState<Range>(LATEST)
  const [page, setPage] = useState(1)
  const data = useQuery({ ...runsQuery(conv, range, page), placeholderData: keepPreviousData }).data
  return (
    <>
      <div className="mb-3 grid gap-2 md:flex md:flex-wrap md:items-center">
        <select aria-label="Conversion" value={conv} onChange={(e) => { setConv(Number(e.target.value)); setPage(1) }} className={cn(INPUT, 'appearance-auto md:w-80')}>
          <option value={0}>All conversions</option>
          {convs.map((c) => <option key={c.id} value={c.id}>{c.fromName} → {c.toName}</option>)}
        </select>
        <RangePicker full value={range} onChange={(r) => { setRange(r); setPage(1) }} />
        <span className="flex gap-1.5 md:ml-auto">
          {[['csv', 'CSV'], ['xlsx', 'Excel']].map(([fmt, l]) => (
            <a key={fmt} download href={`/api/v1/admin/conversions/runs?format=${fmt}&conversion=${conv}${rangeQS(range)}`}
              className="flex h-10 items-center gap-1.5 rounded-lg border border-border bg-card px-3 text-sm font-medium hover:bg-muted"><Download size={15} /> {l}</a>
          ))}
        </span>
      </div>
      <DataList label="Conversion runs" rows={data?.items ?? []} rowKey={(r) => r.id}
        empty={<EmptyState title={data ? 'No runs' : 'Loading…'}>{data && 'Nothing converted for this filter.'}</EmptyState>}
        footer={data && <Pager page={data.page} pageSize={data.pageSize} total={data.total} onPage={setPage} />}
        columns={[
          { key: 'what', header: 'Conversion', cell: (r) => <span className="font-semibold">{r.fromName} → {r.toName}</span> },
          { key: 'out', header: 'Out', num: true, cell: (r) => <b className="text-danger">−{qtyText(r.fromQty)} <small className="font-normal text-faint">{r.fromUnit}</small></b> },
          { key: 'in', header: 'In', num: true, cell: (r) => <b className="text-success">+{qtyText(r.toQty)} <small className="font-normal text-faint">{r.toUnit}</small></b> },
          { key: 'by', header: 'By', cell: (r) => r.userName },
          { key: 'when', header: 'When', cell: (r) => <span className="whitespace-nowrap text-muted-foreground">{new Date(r.createdAt).toLocaleString([], { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })}</span> },
        ]} />
    </>
  )
}
