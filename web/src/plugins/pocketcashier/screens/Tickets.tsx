import { Send } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { api } from '@/api/client'
import type { HeldSale, TicketCashier } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { ticketItems } from '@/features/till/cart/held'
import type { TillState } from '@/features/till/cart/types'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { CARD } from '../bits'

const POLL = 5_000
const who = (t: HeldSale) => t.label || 'Walk-in'

function beep() {
  try {
    const a = new AudioContext()
    const o = a.createOscillator()
    o.connect(a.createGain() as never).connect(a.destination as never)
    o.start()
    o.stop(a.currentTime + 0.15)
    o.onended = () => void a.close()
  } catch { /* no sound on this phone: the list still shows */ }
}

/** Tickets waiting, while the PC answers: a sales person needs the PC, so offline
 * there are none. A new one for this person beeps; bump asks again at once. */
export function useTickets(live: boolean, bump = 0): HeldSale[] {
  const [tickets, setTickets] = useState<HeldSale[]>([])
  const seen = useRef<Set<number> | null>(null)
  useEffect(() => {
    if (!live) {
      setTickets([])
      seen.current = null
      return
    }
    let stopped = false
    const look = () => void api<HeldSale[]>('/till/tickets').then((rows) => {
      if (stopped) return
      setTickets(rows)
      const mine = rows.filter((t) => t.forMe)
      const before = seen.current
      seen.current = new Set(mine.map((t) => t.id))
      if (before && mine.some((t) => !before.has(t.id))) beep()
    }, () => {})
    look()
    const every = setInterval(look, POLL)
    return () => { stopped = true; clearInterval(every) }
  }, [live, bump])
  return tickets
}

/** Takes the ticket for this till. Over a cart with items it refuses, as the PC does. */
async function take(t: HeldSale, cart: TillState): Promise<{ held?: HeldSale; error?: string }> {
  if (cart.lines.length) return { error: 'Finish or hold the current sale first' }
  try {
    return { held: await api<HeldSale>(`/till/tickets/${t.id}/open`, { method: 'POST' }) }
  } catch (e) {
    return { error: e instanceof Error ? e.message : 'The PC could not do this' }
  }
}

/** The list above the screen: every ticket waiting, this person's first. */
export function TicketStrip({ tickets, sym, cart, onOpen }: { tickets: HeldSale[]; sym: string; cart: TillState; onOpen: (t: HeldSale) => void }) {
  const [error, setError] = useState('')
  if (!tickets.length) return null
  const sorted = [...tickets].sort((a, b) => Number(b.forMe ?? false) - Number(a.forMe ?? false))
  return (
    <div role="group" aria-label="Tickets" className="flex max-h-36 flex-col gap-1.5 overflow-y-auto px-3 pb-2">
      <span className="flex items-center gap-1.5 text-[12.5px] font-semibold"><Send size={14} /> Tickets waiting</span>
      {error && <p role="alert" className="text-[13px] font-semibold text-danger">{error}</p>}
      {sorted.map((t) => (
        <button key={t.id} type="button" aria-label={`Ticket ${t.ticketNo}, ${who(t)}`}
          onClick={() => void take(t, cart).then((r) => { setError(r.error ?? ''); if (r.held) onOpen(r.held) })}
          className={cn(CARD, 'flex min-h-12 items-center gap-2.5 px-3 py-1.5 text-left', t.forMe && 'border-primary')}>
          <b className="font-display text-xl tabular-nums">{t.ticketNo}</b>
          <span className="min-w-0 flex-1">
            <span className="flex justify-between gap-2 text-[13px] font-semibold">
              <span className="truncate">{who(t)} · from {t.preparedByName}</span>
              <span className="shrink-0 tabular-nums">{money(sym, t.total)}</span>
            </span>
            <small className="block truncate text-muted-foreground">{ticketItems(t)}</small>
          </span>
        </button>
      ))}
    </div>
  )
}

/** Before a till closes, nothing may be left waiting on it. */
export function TicketsBeforeClose({ tickets, sym, cart, onOpen, onChanged, onClose }: {
  tickets: HeldSale[]; sym: string; cart: TillState; onOpen: (t: HeldSale) => void; onChanged: () => void; onClose: () => void
}) {
  const [others, setOthers] = useState<TicketCashier[]>([])
  const [error, setError] = useState('')
  useEffect(() => {
    void api<TicketCashier[]>('/till/cashiers').then((all) => setOthers(all.filter((c) => !tickets.some((t) => t.forCashierName === c.name))), () => {})
  }, [tickets])
  const act = (path: string, json?: object) => {
    setError('')
    void api(path, { method: 'POST', ...(json ? { json } : {}) }).catch((e: Error) => setError(e.message)).finally(onChanged)
  }
  return (
    <div className="flex flex-col gap-2">
      <h3 className="font-display text-lg font-bold">Tickets waiting on your till</h3>
      <p className="text-[13px] text-muted-foreground">Deal with these first, then you can count the drawer.</p>
      {error && <p role="alert" className="text-[13px] font-semibold text-danger">{error}</p>}
      <ul aria-label="Tickets to deal with" className="flex flex-col gap-2">
        {tickets.map((t) => (
          <li key={t.id} className={cn(CARD, 'flex flex-col gap-2 p-3')}>
            <span className="flex items-center gap-2.5 text-sm font-semibold">
              <b className="font-display text-xl tabular-nums">{t.ticketNo}</b>
              <span className="min-w-0 flex-1 truncate">{who(t)} · from {t.preparedByName}</span>
              <span className="tabular-nums">{money(sym, t.total)}</span>
            </span>
            <small className="truncate text-muted-foreground">{ticketItems(t)}</small>
            <span className="flex flex-wrap gap-2">
              <Button className="h-11" onClick={() => void take(t, cart).then((r) => { setError(r.error ?? ''); if (r.held) { onOpen(r.held); onClose() } })}>Open and sell</Button>
              {others.map((c) => <Button key={c.id} variant="secondary" className="h-11" onClick={() => act(`/till/tickets/${t.id}/pass`, { cashierId: c.id })}>Pass to {c.name}</Button>)}
              <Button variant="secondary" className="h-11" onClick={() => act(`/till/tickets/${t.id}/back`)}>Back to {t.preparedByName}</Button>
            </span>
          </li>
        ))}
      </ul>
    </div>
  )
}
