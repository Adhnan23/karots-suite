import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Send } from 'lucide-react'
import { useCallback, useEffect, useRef } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { HeldSale } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { cashiersQuery, configQuery, errMsg, heldQuery, ticketsQuery } from '../api'
import { ticketItems } from '../cart/held'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'

const beep = () => {
  try {
    const a = new AudioContext()
    const o = a.createOscillator()
    o.connect(a.createGain()).connect(a.destination)
    o.start()
    o.stop(a.currentTime + 0.15)
    o.onended = () => void a.close()
  } catch { /* no sound on this PC: the popup and the list still show */ }
}

const who = (t: HeldSale) => t.label || 'Walk-in'

/** Open a ticket into this till's cart. The server hands it to one till only. */
export function useOpenTicket() {
  const { state, dispatch } = useTill()
  const qc = useQueryClient()
  const navigate = useNavigate()
  return useCallback(async (id: number) => {
    if (state.lines.length) {
      toast.error('Finish or hold the current sale first')
      return false
    }
    let opened = false
    try {
      const t = await api<HeldSale>(`/till/tickets/${id}/open`, { method: 'POST' })
      dispatch({ type: 'resume', held: t })
      opened = true
      void navigate({ to: '/till', search: {} })
    } catch (e) {
      toast.error(errMsg(e))
    }
    void qc.invalidateQueries({ queryKey: ticketsQuery.queryKey })
    return opened
  }, [state.lines.length, dispatch, qc, navigate])
}

/** The list under the cart: every ticket waiting, this till's first. A sales person sees their own, to take back. */
export function TicketStrip() {
  const cfg = useSuspenseQuery(configQuery).data
  const tickets = useQuery(ticketsQuery).data ?? []
  const open = useOpenTicket()
  const qc = useQueryClient()
  // A ticket that came back from the last till closing shows in the sender's Held at once.
  useEffect(() => { void qc.invalidateQueries({ queryKey: heldQuery.queryKey }) }, [tickets.length, qc])
  if (!tickets.length) return null
  const sorted = [...tickets].sort((a, b) => Number(b.forMe ?? false) - Number(a.forMe ?? false))
  return (
    <div role="group" aria-label="Tickets" className="flex max-h-44 flex-col gap-1.5 overflow-auto border-t border-border bg-brand-soft px-3 py-2.5">
      <span className="flex items-center gap-1.5 text-[12.5px] font-semibold"><Send size={14} /> {cfg.sendsToCashier ? 'Sent · tap to take back' : 'Tickets'}</span>
      {sorted.map((t) => (
        <button key={t.id} type="button" onClick={() => void open(t.id)} aria-label={`Ticket ${t.ticketNo}, ${who(t)}`}
          className={cn('flex items-center gap-2.5 rounded-[var(--radius-sm)] border bg-card px-2.5 py-1.5 text-left text-foreground', t.forMe ? 'border-selected' : 'border-border')}>
          <b className="font-display text-xl tabular-nums">{t.ticketNo}</b>
          <span className="min-w-0 flex-1">
            <span className="flex justify-between gap-2 text-[12.5px] font-semibold">
              <span className="truncate">{who(t)} · {cfg.sendsToCashier ? `for ${t.forCashierName}` : `from ${t.preparedByName}${t.forMe ? '' : ` · for ${t.forCashierName}`}`}</span>
              <span className="shrink-0 tabular-nums">{money(cfg.symbol, t.total)}</span>
            </span>
            <small className="block truncate text-[11.5px] text-muted-foreground">{ticketItems(t)}</small>
          </span>
        </button>
      ))}
    </div>
  )
}

/** A cashier's popup when a ticket reaches their till, on any till tab. It never
 * blocks the sale in progress, and the ticket stays in the list until it's opened. */
export function TicketPopups() {
  const cfg = useSuspenseQuery(configQuery).data
  const tickets = useQuery({ ...ticketsQuery, enabled: !cfg.sendsToCashier }).data
  const open = useOpenTicket()
  const seen = useRef<Set<number> | null>(null)
  const live = useRef(open)
  useEffect(() => { live.current = open })
  useEffect(() => {
    if (!tickets) return
    const mine = tickets.filter((t) => t.forMe)
    const before = seen.current
    seen.current = new Set(mine.map((t) => t.id))
    if (!before) return // what was already waiting at sign-in is in the list
    const fresh = mine.filter((t) => !before.has(t.id))
    if (fresh.length) beep()
    for (const t of fresh) {
      toast.info(`Ticket ${t.ticketNo} · ${who(t)} · from ${t.preparedByName}`, {
        description: `${ticketItems(t)} · ${money(cfg.symbol, t.total)}`,
        duration: 10_000,
        action: { label: 'Open', onClick: () => void live.current(t.id) },
      })
    }
  }, [tickets, cfg.symbol])
  return null
}

/** Before a till closes: nothing may be left waiting on it. */
export function TicketsBeforeClose({ tickets }: { tickets: HeldSale[] }) {
  const cfg = useSuspenseQuery(configQuery).data
  const others = (useQuery(cashiersQuery).data ?? []).filter((c) => !tickets.some((t) => t.forCashierName === c.name))
  const open = useOpenTicket()
  const ui = useTillUi()
  const qc = useQueryClient()
  const act = (path: string, json?: object) =>
    api(path, { method: 'POST', ...(json ? { json } : {}) })
      .catch((e) => toast.error(errMsg(e)))
      .finally(() => {
        void qc.invalidateQueries({ queryKey: ticketsQuery.queryKey })
        void qc.invalidateQueries({ queryKey: heldQuery.queryKey })
      })
  return (
    <>
      <DialogHeader>
        <DialogTitle>Tickets waiting on your till</DialogTitle>
        <DialogDescription>Deal with these first, then you can count the drawer.</DialogDescription>
      </DialogHeader>
      <ul aria-label="Tickets to deal with" className="flex flex-col gap-2">
        {tickets.map((t) => (
          <li key={t.id} className="flex flex-col gap-2 rounded-[var(--radius-sm)] border border-border p-3">
            <span className="flex items-center gap-2.5 text-sm font-semibold">
              <b className="font-display text-xl tabular-nums">{t.ticketNo}</b>
              <span className="min-w-0 flex-1 truncate">{who(t)} · from {t.preparedByName}</span>
              <span className="tabular-nums">{money(cfg.symbol, t.total)}</span>
            </span>
            <small className="truncate text-muted-foreground">{ticketItems(t)}</small>
            <span className="flex flex-wrap gap-2">
              <Button onClick={() => void open(t.id).then((ok) => ok && ui.close())}>Open and sell</Button>
              {others.map((c) => <Button key={c.id} variant="secondary" onClick={() => void act(`/till/tickets/${t.id}/pass`, { cashierId: c.id })}>Pass to {c.name}</Button>)}
              <Button variant="secondary" onClick={() => void act(`/till/tickets/${t.id}/back`)}>Back to {t.preparedByName}</Button>
            </span>
          </li>
        ))}
      </ul>
    </>
  )
}
