import { ShieldCheck, ShieldX } from 'lucide-react'
import { useState } from 'react'
import { api, ApiError } from '@/api/client'
import type { WarrantyReceiptLine, WarrantyReceiptView, WarrantyUnitRow } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { REASONS } from '@/features/counter/warranty/reasons'
import { printWarrantySlip } from '@/features/till/print'
import { FIELD } from '@/features/till/ui'
import { num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { CARD } from '../../bits'
import { findSales, findUnit } from '../../store/recent'
import type { Action } from '../../store/outbox'
import { sendCall, type CallInput } from '../../sync'
import { AskPrint } from '../AskPrint'
import { CounterShell, type CounterProps } from './CounterShell'

type Found = { unit: WarrantyUnitRow } | { receipt: WarrantyReceiptView } | { none: true }

const date = (iso: string) => new Date(iso).toLocaleDateString([], { dateStyle: 'medium' })
const callsTo = (out: Action[], suffix: string) => out.filter((a) => a.kind === 'call').map((a) => a.body as CallInput).filter((c) => c.path.endsWith(suffix)).map((c) => c.body as { unitId?: number; saleId?: number; productId?: number })

async function localLook(text: string): Promise<Found> {
  const unit = await findUnit(text)
  if (unit) return { unit }
  const hit = (await findSales(text, 5)).find((s) => s.receiptNo.toLowerCase() === text.toLowerCase())
  return hit?.warranty?.receipt ? { receipt: hit.warranty.receipt } : { none: true }
}

async function liveLook(text: string): Promise<Found> {
  const missing = (e: unknown) => e instanceof ApiError && e.status === 404
  try {
    return { unit: await api<WarrantyUnitRow>(`/till/warranty/serial/${encodeURIComponent(text)}`) }
  } catch (e) {
    if (!missing(e)) return localLook(text)
  }
  try {
    return { receipt: await api<WarrantyReceiptView>(`/till/warranty/receipt?no=${encodeURIComponent(text)}`) }
  } catch (e) {
    return missing(e) ? { none: true } : localLook(text)
  }
}

/** Swap a faulty unit, by its serial or its receipt, on or off the PC. */
export function Warranty(p: CounterProps) {
  const [text, setText] = useState('')
  const [found, setFound] = useState<Found | null>(null)
  const [notice, setNotice] = useState('')
  const done = (n: string) => { setNotice(n); setFound(null); setText(''); p.onSent() }
  async function look(e: { preventDefault(): void }) {
    e.preventDefault()
    const t = text.trim()
    if (!t) return
    setNotice('')
    setFound(await (p.live ? liveLook(t) : localLook(t)))
  }
  return (
    <CounterShell title="Warranty" sub={p.live ? undefined : 'Units and receipts of the last 30 days'} onBack={p.onBack}>
      {notice && <p role="status" className="px-1 text-sm font-semibold text-success">{notice}</p>}
      <form onSubmit={look} autoComplete="off" className="flex gap-2">
        <input aria-label="Serial or receipt" placeholder="Serial or receipt number" value={text} onChange={(e) => setText(e.target.value)} className={cn(FIELD, 'h-12 min-w-0 flex-1')} />
        <Button type="submit" className="h-12 px-5">Find</Button>
      </form>
      {found && 'none' in found && (
        <p className="m-auto p-8 text-center text-faint">{p.live ? `Nothing found for “${text}”.` : 'Not in the last 30 days on this phone. Ask them to come back when the PC is on.'}</p>
      )}
      {found && 'unit' in found && <UnitSwap key={found.unit.id} {...p} unit={found.unit} onDone={done} />}
      {found && 'receipt' in found && <ReceiptLines {...p} receipt={found.receipt} onDone={done} />}
    </CounterShell>
  )
}

function Reasons({ value, onChange }: { value: string; onChange: (r: string) => void }) {
  return (
    <div className="flex flex-wrap gap-1.5">
      {REASONS.map((r) => (
        <button key={r} type="button" aria-pressed={value === r} onClick={() => onChange(value === r ? '' : r)}
          className="min-h-9 rounded-pill border border-border px-3 text-[13px] font-semibold aria-pressed:border-primary aria-pressed:bg-primary aria-pressed:text-primary-foreground">{r}</button>
      ))}
    </div>
  )
}

function UnitSwap({ who, copy, out, unit: u, onDone }: CounterProps & { unit: WarrantyUnitRow; onDone: (n: string) => void }) {
  const [next, setNext] = useState('')
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [ask, setAsk] = useState<{ id: number; notice: string } | null>(null)
  const waiting = callsTo(out, '/till/warranty/replace').some((b) => b.unitId === u.id)
  const canSwap = u.status === 'active' && u.underWarranty && !waiting
  const Icon = u.underWarranty ? ShieldCheck : ShieldX

  async function swap(e: { preventDefault(): void }) {
    e.preventDefault()
    setBusy(true)
    setError('')
    const newSerial = next.trim()
    const r = await sendCall(who.id, { method: 'POST', path: '/api/v1/till/warranty/replace', label: `Warranty swap ${u.serial} to ${newSerial}`, body: { unitId: u.id, newSerial, reason } })
    setBusy(false)
    if (r?.status === 'failed') return setError(r.note || 'The PC could not do this')
    const verb = `Swapped ${u.serial} for ${newSerial}`
    if (!r) return onDone(`${verb} on this phone. It sends when the PC is back; no slip yet.`)
    const notice = `${verb}. The slip prints at the counter.`
    if (r.resultId && copy.askToPrint) return setAsk({ id: r.resultId, notice })
    if (r.resultId) void printWarrantySlip(r.resultId).catch(() => {})
    onDone(notice)
  }

  return (
    <section className="flex flex-col gap-3">
      <div className={cn(CARD, 'px-4 py-3')}>
        <b className="block text-[15px]">{u.productName}</b>
        <small className="text-muted-foreground tabular-nums">{u.serial} · sold {date(u.soldAt)}{u.customerName && ` · ${u.customerName}`}</small>
        <p className={cn('mt-2 flex items-center gap-2 text-[14px] font-semibold', u.underWarranty ? 'text-success' : 'text-danger')}>
          <Icon size={20} />{u.underWarranty ? `Covered: ${u.left}` : 'Out of warranty'}
        </p>
      </div>
      {waiting && <p className="px-1 text-[13px] font-semibold text-warning">This unit is already swapped on this phone, waiting for the PC.</p>}
      {u.status !== 'active' && <p className="px-1 text-[13px] text-muted-foreground">This unit was already replaced.</p>}
      {canSwap && (
        <form onSubmit={swap} autoComplete="off" className="flex flex-col gap-3">
          <input aria-label="New unit serial" placeholder="New unit's serial" value={next} onChange={(e) => setNext(e.target.value)} className={cn(FIELD, 'h-12 w-full')} />
          <Reasons value={reason} onChange={setReason} />
          {error && <p role="alert" className="px-1 text-[13px] font-semibold text-danger">{error}</p>}
          <Button type="submit" size="lg" disabled={busy || !next.trim()} className="h-12 text-[15px]">Swap the unit</Button>
        </form>
      )}
      {ask && <AskPrint onPrint={() => void printWarrantySlip(ask.id).catch(() => {})} onClose={() => onDone(ask.notice)} />}
    </section>
  )
}

function ReceiptLines({ who, out, receipt: v, onDone }: CounterProps & { receipt: WarrantyReceiptView; onDone: (n: string) => void }) {
  return (
    <section className="flex flex-col gap-2">
      <p className="px-1 text-[13px] text-muted-foreground">{v.receiptNo} · sold {date(v.soldAt)}{v.customerName && ` · ${v.customerName}`}</p>
      {v.lines.length === 0 && <p className="m-auto p-8 text-center text-faint">Nothing on this receipt carries a warranty.</p>}
      {v.lines.map((l) => <ReceiptLine key={l.productId} who={who} out={out} v={v} l={l} onDone={onDone} />)}
    </section>
  )
}

function ReceiptLine({ who, out, v, l, onDone }: { who: CounterProps['who']; out: Action[]; v: WarrantyReceiptView; l: WarrantyReceiptLine; onDone: (n: string) => void }) {
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const used = callsTo(out, '/till/warranty/receipt-replace').filter((b) => b.saleId === v.saleId && b.productId === l.productId).length
  const remaining = num(l.remaining) - used
  async function replace() {
    setBusy(true)
    setError('')
    const r = await sendCall(who.id, { method: 'POST', path: '/api/v1/till/warranty/receipt-replace', label: `Warranty replacement: ${l.name} on ${v.receiptNo}`, body: { saleId: v.saleId, productId: l.productId, reason } })
    setBusy(false)
    if (r?.status === 'failed') return setError(r.note || 'The PC could not do this')
    onDone(`Give the customer a new ${l.name}.${r ? '' : ' It sends when the PC is back.'}`)
  }
  return (
    <div className={cn(CARD, 'flex flex-col gap-2 px-4 py-3')}>
      <b className="text-[15px]">{l.name}</b>
      <small className="text-muted-foreground tabular-nums">{qtyText(l.qty)} {l.unitAbbr} sold · {remaining} can still be replaced · until {date(l.until)}</small>
      {l.underWarranty && remaining > 0 ? (
        <>
          <Reasons value={reason} onChange={setReason} />
          {error && <p role="alert" className="text-[13px] font-semibold text-danger">{error}</p>}
          <Button disabled={busy} onClick={() => void replace()} className="h-12">Replace one</Button>
        </>
      ) : <p className="text-[13px] font-semibold text-danger">{l.underWarranty ? 'All replaced already' : 'Out of warranty'}</p>}
    </div>
  )
}
