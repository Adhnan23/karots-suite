import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { Home } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { configQuery, errMsg } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { money } from '@/lib/money'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import type { TillApi } from '@/plugins/slots'
import { type JobAsk, ownUse, quote } from './api'
import { customLine, meteredLine, offerOf, offersFor, settle, sizes, startPick, type Pick } from './till'
import type { TillService } from './types'

const OPT = 'h-11 min-w-[64px] rounded-[var(--radius-sm)] border border-border bg-card px-4 text-[15px] font-bold aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground disabled:opacity-35'

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <span className="w-[68px] shrink-0 text-[12px] font-bold uppercase tracking-wide text-faint">{label}</span>
      <div role="group" aria-label={label} className="flex flex-wrap gap-1.5">{children}</div>
    </div>
  )
}

/** One metered service: size, colour, side and copies, priced live; Enter adds. */
function Metered({ s, till }: { s: TillService; till: TillApi }) {
  const sym = useQuery(configQuery).data?.symbol ?? ''
  const [pick, setPick] = useState<Pick>(() => startPick(s))
  const [text, setText] = useState('1')
  const [confirm, setConfirm] = useState(false)
  const [busy, setBusy] = useState(false)
  const input = useRef<HTMLInputElement>(null)
  const copies = Math.max(0, Math.floor(Number(text) || 0))
  const key = JSON.stringify({ service: s.id, ...pick, copies })
  const settled = useDebounced(key, 120)
  const ask = JSON.parse(settled) as JobAsk
  const q = useQuery({ queryKey: ['x', 'documents', 'quote', ask], queryFn: () => quote(ask), enabled: ask.copies > 0, placeholderData: keepPreviousData, retry: false })
  const fresh = !!q.data && !q.isPlaceholderData && settled === key
  const offers = offersFor(s, pick.size)
  const has = (color: boolean, double: boolean) => offers.some((o) => o.color === color && o.double === double)
  const bands = offerOf(s, pick)?.bands ?? []
  const next = bands.find((b) => b.minQty > copies)
  const choose = (p: Partial<Pick>) => {
    setPick(settle(s, { ...pick, ...p }))
    input.current?.focus()
  }
  useEffect(() => input.current?.select(), [])

  // Enter straight after typing beats the debounce: price exactly what's typed.
  const add = async () => {
    if (copies <= 0 || busy) return
    setBusy(true)
    try {
      const qd = fresh ? q.data! : await quote({ service: s.id, ...pick, copies })
      till.addServiceLine(meteredLine(s, pick, copies, qd))
      toast.success(`${qd.description} ×${copies} added`)
      setText('1')
      input.current?.focus()
      input.current?.select()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  const shopUse = async () => {
    setConfirm(false)
    try {
      await ownUse({ service: s.id, ...pick, copies })
      toast.success('Shop use recorded — the paper is off stock')
      setText('1')
    } catch (e) {
      toast.error(errMsg(e))
    }
  }

  return (
    <div className="flex flex-col gap-3.5">
      {sizes(s).length > 1 || sizes(s)[0] ? (
        <Row label="Size">
          {sizes(s).map((z) => <button key={z} type="button" className={OPT} aria-pressed={pick.size === z} onClick={() => choose({ size: z })}>{z}</button>)}
        </Row>
      ) : null}
      <Row label="Ink">
        <button type="button" className={OPT} aria-pressed={!pick.color} disabled={!has(false, pick.double) && !has(false, !pick.double)} onClick={() => choose({ color: false })}>B&amp;W</button>
        <button type="button" className={OPT} aria-pressed={pick.color} disabled={!has(true, pick.double) && !has(true, !pick.double)} onClick={() => choose({ color: true })}>Colour</button>
      </Row>
      <Row label="Sides">
        <button type="button" className={OPT} aria-pressed={!pick.double} disabled={!has(pick.color, false)} onClick={() => choose({ double: false })}>1-side</button>
        <button type="button" className={OPT} aria-pressed={pick.double} disabled={!has(pick.color, true)} onClick={() => choose({ double: true })}>2-side</button>
      </Row>
      <Row label="Copies">
        <input ref={input} aria-label="Number of copies" inputMode="numeric" autoComplete="off" value={text} name="doc-copies"
          onChange={(e) => setText(e.target.value.replace(/\D/g, '').slice(0, 6))}
          onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); void add() } }}
          className={cn(FIELD, 'h-11 w-28 text-center font-display text-[20px] font-bold tabular-nums')} />
        {[10, 50, 100].map((n) => (
          <button key={n} type="button" className={cn(OPT, 'min-w-0 px-3 text-[13px] font-semibold')} onClick={() => { setText(String(copies + n)); input.current?.focus() }}>+{n}</button>
        ))}
      </Row>
      {bands.length > 1 && (
        <div aria-label="Price bands" className="flex flex-wrap items-center gap-1.5 pl-[76px] text-[12.5px]">
          {bands.map((b, i) => {
            const on = b.minQty <= copies && (bands[i + 1]?.minQty ?? Infinity) > copies
            return (
              <span key={b.minQty} className={cn('rounded-full border px-2.5 py-0.5 tabular-nums', on ? 'border-brand bg-brand-soft font-bold text-foreground' : 'border-border text-muted-foreground')}>
                {b.minQty}+ · {money(sym, b.price)}
              </span>
            )
          })}
          {next && copies > 0 && <span className="text-muted-foreground">{next.minQty - copies} more for {money(sym, next.price)} each</span>}
        </div>
      )}
      {!confirm && <div className="flex flex-wrap items-center gap-3 rounded-[var(--radius-sm)] border border-border bg-muted px-4 py-3">
        <div className="min-w-0 flex-1">
          {q.error && copies > 0 ? <p className="text-[13px] font-semibold text-danger">{errMsg(q.error)}</p> : (
            <>
              <b className="block font-display text-[24px] leading-tight tabular-nums">{q.data && copies > 0 ? money(sym, q.data.total) : '—'}</b>
              {q.data && copies > 0 && (
                <small className="text-[12.5px] text-muted-foreground tabular-nums">
                  {copies} × {money(sym, q.data.unitPrice)} · {q.data.sheets} sheet{q.data.sheets === 1 ? '' : 's'}
                </small>
              )}
            </>
          )}
        </div>
        <Button variant="ghost" className="h-12 gap-1.5 px-3" disabled={!fresh || copies <= 0} onClick={() => setConfirm(true)}><Home size={16} /> Shop use</Button>
        <Button className="h-12 px-6 text-[15px] font-bold" disabled={busy || copies <= 0 || (!!q.error && settled === key)} onClick={() => void add()}>Add to sale ⏎</Button>
      </div>}
      {confirm && (
        <div role="alertdialog" aria-label="Record as shop use?" className="flex flex-wrap items-center gap-3 rounded-[var(--radius-sm)] border border-warning bg-[color-mix(in_srgb,var(--warning)_12%,var(--card))] px-4 py-3">
          <p className="min-w-0 flex-1 text-[13.5px]"><b>Record as shop use?</b> No customer, no charge — the paper and toner come off stock as the shop's own use.</p>
          <Button variant="ghost" className="h-10" onClick={() => setConfirm(false)}>Cancel</Button>
          <Button autoFocus className="h-10 font-bold" onClick={() => void shopUse()}>Record {copies} as shop use</Button>
        </div>
      )}
    </div>
  )
}

/** A custom job (CV, photo edit, poster): the typed price and a note. */
function Custom({ s, till }: { s: TillService; till: TillApi }) {
  const [price, setPrice] = useState('')
  const [note, setNote] = useState('')
  const ok = Number(price) > 0
  const add = () => {
    if (!ok) return
    till.addServiceLine(customLine(s, Number(price), note))
    toast.success(`${s.name} added`)
    setPrice('')
    setNote('')
  }
  return (
    <form className="flex max-w-lg flex-col gap-3" onSubmit={(e) => { e.preventDefault(); add() }}>
      <label className="flex flex-col gap-1 text-[12px] font-bold uppercase tracking-wide text-faint">Price
        <input autoFocus inputMode="decimal" autoComplete="off" name="doc-price" placeholder="0.00" value={price} onChange={(e) => setPrice(e.target.value.replace(/[^\d.]/g, ''))}
          className={cn(FIELD, 'h-12 font-display text-[20px] font-bold tabular-nums text-foreground')} />
      </label>
      <label className="flex flex-col gap-1 text-[12px] font-bold uppercase tracking-wide text-faint">Note on the receipt (optional)
        <input autoComplete="off" name="doc-note" maxLength={60} placeholder="e.g. Nimal — 2 pages" value={note} onChange={(e) => setNote(e.target.value)} className={cn(FIELD, 'normal-case tracking-normal')} />
      </label>
      <Button type="submit" className="h-12 self-start px-6 text-[15px] font-bold" disabled={!ok}>Add to sale ⏎</Button>
    </form>
  )
}

export function JobPanel({ s, till }: { s: TillService; till: TillApi }) {
  if (s.kind === 'custom') return <Custom s={s} till={till} />
  if (!s.offers.length) return <p className="text-[14px] text-muted-foreground">No prices set yet — add them in Apps → Print &amp; Copy.</p>
  return <Metered s={s} till={till} />
}
