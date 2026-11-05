import { ArrowRight } from 'lucide-react'
import { useState } from 'react'
import type { ConversionRow } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { yieldNote } from '@/features/admin/conversions/yield'
import { FIELD } from '@/features/till/ui'
import { num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { CARD } from '../../bits'
import { sendCall } from '../../sync'
import { CounterShell, type CounterProps } from './CounterShell'

/** Break a pack into units so the pieces can be sold, on or off the PC. */
export function Convert(p: CounterProps) {
  const [id, setId] = useState<number | null>(null)
  const [notice, setNotice] = useState('')
  const rows = p.copy.conversions ?? []
  const cur = rows.find((r) => r.id === id)
  if (cur) return <RunForm key={cur.id} {...p} r={cur} onDone={(n) => { setNotice(n); setId(null); p.onSent() }} onBack={() => setId(null)} />
  return (
    <CounterShell title="Convert" onBack={p.onBack}>
      {notice && <p role="status" className="px-1 text-sm font-semibold text-success">{notice}</p>}
      {rows.length === 0 && <p className="m-auto p-8 text-center text-faint">No packs set up. An admin adds them on the PC.</p>}
      {rows.map((r) => (
        <button key={r.id} type="button" onClick={() => { setNotice(''); setId(r.id) }} className={cn(CARD, 'flex min-h-14 items-center gap-2 px-4 py-2.5 text-left')}>
          <span className="min-w-0 flex-1">
            <b className="flex items-center gap-1.5 text-[14.5px]"><span className="truncate">{r.fromName}</span> <ArrowRight size={14} className="shrink-0 text-faint" /> <span className="truncate">{r.toName}</span></b>
            <small className="text-muted-foreground tabular-nums">1 {r.fromUnit} = {qtyText(r.ratio)} {r.toUnit}</small>
          </span>
          <span className="shrink-0 text-[13px] font-semibold tabular-nums">{qtyText(r.fromStock)} {r.fromUnit}</span>
        </button>
      ))}
    </CounterShell>
  )
}

function RunForm({ who, r, onDone, onBack }: CounterProps & { r: ConversionRow; onDone: (n: string) => void }) {
  const [qty, setQty] = useState('1')
  const [got, setGot] = useState<string | null>(null) // null = what the ratio says
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const n = num(qty)
  const expected = n * num(r.ratio)
  const out = got === null ? expected : num(got)
  const note = yieldNote(n, expected, out, r.toUnit)
  const over = n > num(r.fromStock)

  async function run(e: { preventDefault(): void }) {
    e.preventDefault()
    setBusy(true)
    setError('')
    const what = `${qtyText(n)} ${r.fromUnit} ${r.fromName} to ${qtyText(out)} ${r.toUnit} ${r.toName}`
    const res = await sendCall(who.id, { method: 'POST', path: `/api/v1/till/conversions/${r.id}/run`, label: `Convert ${what}`, body: note ? { quantity: qty, toQty: String(out) } : { quantity: qty } })
    setBusy(false)
    if (res?.status === 'failed') return setError(res.note || 'The PC could not do this')
    onDone(`Opened ${what}.${res ? '' : ' It sends when the PC is back.'}`)
  }

  return (
    <CounterShell title={`Break open ${r.fromName}`} sub={`${qtyText(r.fromStock)} ${r.fromUnit} on hand`} onBack={onBack}>
      <form onSubmit={run} autoComplete="off" className="flex flex-col gap-3">
        <label className="flex flex-col gap-1 text-[13px] font-semibold text-muted-foreground">
          How many {r.fromUnit} to open?
          <input aria-label="Packs to open" inputMode="decimal" value={qty} onChange={(e) => { setQty(e.target.value.replace(/[^\d.]/g, '')); setGot(null) }} className={cn(FIELD, 'h-14 w-full font-display text-2xl font-bold tabular-nums')} />
        </label>
        {over && <p className="px-1 text-[13px] font-semibold text-warning">That's more than the {qtyText(r.fromStock)} shown. It still saves, for the PC to check.</p>}
        <label className="flex flex-col gap-1 text-[13px] font-semibold text-muted-foreground">
          Got ({r.toUnit})
          <input aria-label="Got" inputMode="decimal" value={got ?? qtyText(expected)} onChange={(e) => setGot(e.target.value.replace(/[^\d.]/g, ''))} className={cn(FIELD, 'h-14 w-full font-display text-2xl font-bold tabular-nums', note && 'border-warning')} />
          <small className={note ? 'text-warning' : 'font-normal text-faint'}>{note || 'What really came out: leave as is if it matches.'}</small>
        </label>
        {r.note && <p className="px-1 text-[13px] text-muted-foreground">{r.note}</p>}
        {error && <p role="alert" className="px-1 text-[13px] font-semibold text-danger">{error}</p>}
        <Button type="submit" size="lg" disabled={busy || !(n > 0) || !(out > 0)} className="h-12 text-[15px]">Open {qtyText(n || 0)} {r.fromUnit}: get {qtyText(out || 0)} {r.toUnit}</Button>
      </form>
    </CounterShell>
  )
}
