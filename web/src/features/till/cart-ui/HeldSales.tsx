import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { PauseCircle, X } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { money } from '@/lib/money'
import { configQuery, errMsg, heldQuery } from '../api'
import { useTillUi } from '../TillUi'
import { useResume } from './useHold'

/** The strip under the cart: held sales as chips (tap to resume). */
export function HeldStrip() {
  const cfg = useSuspenseQuery(configQuery).data
  const held = useQuery(heldQuery).data ?? []
  const resume = useResume()
  const ui = useTillUi()
  if (!held.length) return null
  return (
    <div className="flex items-center gap-2 overflow-hidden border-t border-border px-3.5 py-2.5 text-[12.5px] text-muted-foreground">
      <PauseCircle size={16} className="shrink-0" /> Held
      {held.slice(0, 3).map((h) => (
        <button key={h.id} type="button" onClick={() => void resume(h)}
          className="truncate rounded-pill border border-border bg-muted px-2.5 py-1 text-[12px] font-semibold text-foreground">
          {h.label || 'Sale'} · {money(cfg.symbol, h.total)}
        </button>
      ))}
      {held.length > 3 && <button type="button" className="font-semibold underline-offset-2 hover:underline" onClick={() => ui.open({ kind: 'held' })}>+{held.length - 3} more</button>}
    </div>
  )
}

export function HeldSales() {
  const cfg = useSuspenseQuery(configQuery).data
  const held = useQuery(heldQuery).data ?? []
  const resume = useResume()
  const ui = useTillUi()
  const qc = useQueryClient()
  const remove = (id: number) =>
    api(`/till/held/${id}`, { method: 'DELETE' })
      .then(() => { toast.success('Held sale removed'); return qc.invalidateQueries({ queryKey: heldQuery.queryKey }) })
      .catch((e) => toast.error(errMsg(e)))
  return (
    <Dialog open onOpenChange={(o) => !o && ui.close()}>
      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>Held sales</DialogTitle>
          <DialogDescription>{held.length ? 'Resume one to finish it.' : 'Nothing is held.'}</DialogDescription>
        </DialogHeader>
        <ul className="flex flex-col gap-2">
          {held.map((h) => (
            <li key={h.id} className="flex items-center gap-3 rounded-[var(--radius-sm)] border border-border bg-muted p-3">
              <span className="flex-1">
                <b>{h.label || 'Sale'}</b>
                <small className="block text-muted-foreground">{h.itemCount} {h.itemCount === 1 ? 'line' : 'lines'} · {new Date(h.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</small>
              </span>
              <b className="tabular-nums">{money(cfg.symbol, h.total)}</b>
              <Button size="sm" onClick={() => void resume(h)}>Resume</Button>
              <button type="button" aria-label={`Remove held sale ${h.label}`} onClick={() => void remove(h.id)} className="text-faint hover:text-danger"><X size={16} /></button>
            </li>
          ))}
        </ul>
      </DialogContent>
    </Dialog>
  )
}
