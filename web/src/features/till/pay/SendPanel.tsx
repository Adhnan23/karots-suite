import { useQuery } from '@tanstack/react-query'
import { useRef, useEffect, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { cashiersQuery } from '../api'
import { Kbd, Seg } from '../bits'
import { useHold } from '../cart-ui/useHold'
import { useOpenTicket } from '../cart-ui/Tickets'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'
import { FIELD } from '../ui'
import { CustomerPicker } from './CustomerPicker'
import { Summary } from './Summary'

/** A sales person's Pay panel: the bill, and Send to cashier instead of taking money. */
export function SendPanel() {
  const { state, dispatch } = useTill()
  const hold = useHold()
  const ui = useTillUi()
  const open = useOpenTicket()
  const cashiers = useQuery(cashiersQuery).data
  const [note, setNote] = useState('')
  const [last, setLast] = useState<number | null>(null)
  const undo = useRef(open)
  useEffect(() => { undo.current = open })
  const send = async (forCashier: number) => {
    const t = await hold({ forCashier, note })
    if (!t) return
    setNote('')
    setLast(t.ticketNo ?? null)
    toast.success(`Ticket ${t.ticketNo} sent to ${t.forCashierName}`, {
      action: { label: 'Undo', onClick: () => void undo.current(t.id).then((ok) => ok && setLast(null)) },
    })
  }
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-auto p-3.5">
        <Seg label="Price list" value={state.saleType} options={[['retail', 'Retail'], ['wholesale', 'Wholesale']]}
          onChange={(v) => dispatch({ type: 'saleType', saleType: v })} />
        <CustomerPicker />
        <input aria-label="Name or note" placeholder="Name or note, e.g. blue shirt (optional)" autoComplete="off" value={note}
          onChange={(e) => setNote(e.target.value)} className={`${FIELD} h-10 px-3 text-sm`} />
        <Summary />
        {last !== null && !state.lines.length && (
          <p aria-label="Last ticket" className="rounded-[var(--radius-sm)] bg-brand-soft p-3 text-center text-sm font-semibold">
            Tell the customer: number
            <b className="block font-display text-5xl tabular-nums">{last}</b>
          </p>
        )}
      </div>
      <div className="flex shrink-0 flex-col gap-2 border-t border-border p-3.5">
        {cashiers && !cashiers.length && <p className="text-center text-sm text-muted-foreground">No cashier has opened a till. Hold this sale for now.</p>}
        {cashiers?.map((c) => (
          <Button key={c.id} className="h-[54px] text-base" disabled={!state.lines.length} onClick={() => void send(c.id)}>Send to {c.name}</Button>
        ))}
        <div className="flex gap-2">
          <Button variant="secondary" className="flex-1" onClick={() => void hold()}>Hold <Kbd>F4</Kbd></Button>
          <Button variant="secondary" className="flex-1" disabled={!state.lines.length} onClick={() => ui.open({ kind: 'clearSale' })}>Clear</Button>
        </div>
      </div>
    </div>
  )
}
