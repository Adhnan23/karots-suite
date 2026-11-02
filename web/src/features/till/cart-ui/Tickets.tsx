import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Send } from 'lucide-react'
import { useCallback, useEffect, useRef } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { HeldSale } from '@/api/types.gen'
import { money } from '@/lib/money'
import { configQuery, errMsg, ticketsQuery } from '../api'
import { useTill } from '../TillProvider'

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

/** Open a ticket into this till's cart. The server hands it to one till only. */
export function useOpenTicket() {
  const { state, dispatch } = useTill()
  const qc = useQueryClient()
  const navigate = useNavigate()
  return useCallback(async (id: number) => {
    if (state.lines.length) return void toast.error('Finish or hold the current sale first')
    try {
      const t = await api<HeldSale>(`/till/tickets/${id}/open`, { method: 'POST' })
      dispatch({ type: 'resume', held: t })
      void navigate({ to: '/till', search: {} })
    } catch (e) {
      toast.error(errMsg(e))
    }
    void qc.invalidateQueries({ queryKey: ticketsQuery.queryKey })
  }, [state.lines.length, dispatch, qc, navigate])
}

/** The strip under the cart: tickets waiting (a sales person: their own, to take back). */
export function TicketStrip() {
  const cfg = useSuspenseQuery(configQuery).data
  const tickets = useQuery(ticketsQuery).data ?? []
  const open = useOpenTicket()
  if (!tickets.length) return null
  return (
    <div role="group" aria-label="Tickets" className="flex max-h-32 flex-wrap items-center gap-2 overflow-auto border-t border-border bg-brand-soft px-3.5 py-2.5 text-[12.5px] font-semibold">
      <Send size={16} className="shrink-0" /> {cfg.sendsToCashier ? 'Sent' : 'Tickets'}
      {tickets.map((t) => (
        <button key={t.id} type="button" onClick={() => void open(t.id)}
          className="rounded-pill border border-border bg-card px-2.5 py-1 text-[12px] font-semibold text-foreground">
          #{t.id} {t.label || 'Sale'} · {money(cfg.symbol, t.total)}{!cfg.sendsToCashier && ` · ${t.preparedByName}`}
        </button>
      ))}
    </div>
  )
}

/** A cashier's popup when a ticket arrives, on any till tab. It never blocks the
 * sale in progress, and the ticket stays in the strip until it's opened. */
export function TicketPopups() {
  const cfg = useSuspenseQuery(configQuery).data
  const tickets = useQuery({ ...ticketsQuery, enabled: !cfg.sendsToCashier }).data
  const open = useOpenTicket()
  const seen = useRef<Set<number> | null>(null)
  const live = useRef(open)
  useEffect(() => { live.current = open })
  useEffect(() => {
    if (!tickets) return
    const before = seen.current
    seen.current = new Set(tickets.map((t) => t.id))
    if (!before) return // what was already waiting at sign-in is in the strip
    const fresh = tickets.filter((t) => !before.has(t.id))
    if (fresh.length) beep()
    for (const t of fresh) {
      toast.info(`Ticket #${t.id} from ${t.preparedByName}`, {
        description: `${t.label || 'Sale'} · ${t.itemCount} item${t.itemCount === 1 ? '' : 's'} · ${money(cfg.symbol, t.total)}`,
        duration: 10_000,
        action: { label: 'Open', onClick: () => void live.current(t.id) },
      })
    }
  }, [tickets, cfg.symbol])
  return null
}
