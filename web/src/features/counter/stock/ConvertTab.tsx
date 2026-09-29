import { useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { ArrowRight, Search } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { ConversionRow } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { errMsg } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { conversionsQuery } from '../queries'
import { CounterPage, PaneTitle } from '../ui'


/** Break a pack into units (1 box → 24 pcs) so the till can sell the pieces. */
export function ConvertTab() {
  const rows = useSuspenseQuery(conversionsQuery).data
  const [q, setQ] = useState('')
  const [id, setId] = useState<number | null>(null)
  const shown = rows.filter((r) => `${r.fromName} ${r.toName}`.toLowerCase().includes(q.trim().toLowerCase()))
  const cur = rows.find((r) => r.id === id)
  return (
    <CounterPage flush aside={cur ? <RunForm key={cur.id} r={cur} /> : (
      <p className="m-auto max-w-[260px] p-8 text-center text-faint">Pick what to break open on the left.</p>
    )}>
      <PaneTitle title="Convert" sub={`${rows.length} ${rows.length === 1 ? 'pack' : 'packs'}`} />
      <div className="relative mx-3 mb-2">
        <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
        <input autoFocus aria-label="Find a pack" placeholder="Find a pack or product" autoComplete="off" value={q}
          onChange={(e) => setQ(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && shown[0] && setId(shown[0].id)}
          className={cn(FIELD, 'w-full pl-9')} />
      </div>
      <ul className="flex min-h-0 flex-1 flex-col gap-1 overflow-auto px-2.5 pb-2.5">
        {rows.length === 0 && <li className="m-auto p-8 text-center text-faint">No packs set up. An admin adds them under Products.</li>}
        {shown.map((r) => (
          <li key={r.id}>
            <button type="button" aria-pressed={id === r.id} onClick={() => setId(r.id)} disabled={!(num(r.fromStock) > 0)}
              className="flex w-full items-center gap-2 rounded-[var(--radius-sm)] px-3 py-2 text-left hover:bg-muted disabled:opacity-45 aria-pressed:bg-selected aria-pressed:text-selected-foreground">
              <span className="min-w-0 flex-1">
                <b className="flex items-center gap-1.5 truncate text-[14px]">{r.fromName} <ArrowRight size={14} className="shrink-0 opacity-60" /> {r.toName}</b>
                <small className="opacity-70 tabular-nums">1 {r.fromUnit} = {qtyText(r.ratio)} {r.toUnit}</small>
              </span>
              <span className="text-[13px] font-semibold tabular-nums">{qtyText(r.fromStock)} {r.fromUnit}</span>
            </button>
          </li>
        ))}
      </ul>
    </CounterPage>
  )
}

function RunForm({ r }: { r: ConversionRow }) {
  const qc = useQueryClient()
  const [qty, setQty] = useState('1')
  const [busy, setBusy] = useState(false)
  const n = num(qty)
  const out = n * num(r.ratio)
  const tooMany = n > num(r.fromStock)

  async function run(e: { preventDefault(): void }) {
    e.preventDefault()
    if (!(n > 0) || tooMany) return
    setBusy(true)
    try {
      await api(`/till/conversions/${r.id}/run`, { json: { quantity: qty } })
      toast.success(`Opened ${qtyText(n)} ${r.fromUnit} → ${qtyText(out)} ${r.toUnit} ${r.toName}`)
      setQty('1')
      void qc.invalidateQueries({ queryKey: ['till'] })
    } catch (err) {
      toast.error(errMsg(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={run} autoComplete="off" className="flex flex-col gap-5 overflow-auto p-5">
      <h2 className="font-display text-xl font-bold">Break open {r.fromName}</h2>
      <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-3 text-center">
        <Side name={r.fromName} qty={num(r.fromStock) - (n || 0)} was={r.fromStock} unit={r.fromUnit} />
        <ArrowRight className="text-faint" />
        <Side name={r.toName} qty={num(r.toStock) + (out || 0)} was={r.toStock} unit={r.toUnit} />
      </div>
      <label className="flex flex-col gap-1.5 text-[13px] font-semibold text-muted-foreground">
        How many {r.fromUnit} to open?
        <div className="flex items-center gap-2">
          <input aria-label="Packs to open" inputMode="decimal" value={qty} onChange={(e) => setQty(e.target.value.replace(/[^\d.]/g, ''))}
            className={cn(FIELD, 'h-12 flex-1 font-display text-xl font-bold tabular-nums', tooMany && 'border-danger')} />
          {[1, 2, 5].filter((v) => v <= num(r.fromStock)).map((v) => (
            <button key={v} type="button" onClick={() => setQty(String(v))} className="h-12 min-w-12 rounded-[var(--radius-sm)] border border-border px-3 font-bold hover:bg-muted">{v}</button>
          ))}
        </div>
        {tooMany && <small className="text-danger">Only {qtyText(r.fromStock)} {r.fromUnit} on hand.</small>}
      </label>
      {r.note && <p className="text-[13px] text-muted-foreground">{r.note}</p>}
      <Button type="submit" size="lg" disabled={busy || !(n > 0) || tooMany} className="h-12 text-[15px]">
        Open {qtyText(n || 0)} {r.fromUnit} → get {qtyText(out || 0)} {r.toUnit}
      </Button>
    </form>
  )
}

function Side({ name, qty, was, unit }: { name: string; qty: number; was: string; unit: string }) {
  return (
    <div className="rounded-[var(--radius-sm)] bg-muted px-3 py-3">
      <small className="block truncate text-muted-foreground">{name}</small>
      <b className="font-display text-2xl tabular-nums">{qtyText(qty)}</b> <small>{unit}</small>
      <small className="block text-faint tabular-nums">now {qtyText(was)}</small>
    </div>
  )
}
