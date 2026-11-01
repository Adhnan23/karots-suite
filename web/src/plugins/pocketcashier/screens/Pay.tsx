import { ChevronLeft, Printer, X } from 'lucide-react'
import { useState } from 'react'
import type { TillCopy } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import type { Action } from '@/features/till/cart/store'
import { effectivePayments } from '@/features/till/cart/checkout'
import { balanceDue, changeDue, saleTotal } from '@/features/till/cart/totals'
import type { TillState } from '@/features/till/cart/types'
import { printSale } from '@/features/till/print'
import { FIELD } from '@/features/till/ui'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { CARD, LivePill } from '../bits'
import { payIssue, saleBody } from '../sale'
import type { Who } from '../session'
import { sendNow } from '../sync'
import { CartLines } from './CartBar'
import { PayTenders } from './PayTenders'

type Done = { text: string; change: number; print?: number }

/** Pay on its own page: "Back to items" keeps the cart. The sale goes once (one
 * ref, online or offline); online its receipt prints at the counter, never here. */
export function Pay({ who, copy, cart, dispatch, live, onBack }: { who: Who; copy: TillCopy; cart: TillState; dispatch: (a: Action) => void; live: boolean; onBack: () => void }) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [done, setDone] = useState<Done | null>(null)
  const total = saleTotal(cart)

  async function finish() {
    const issue = payIssue(cart, copy)
    if (issue) return setError(issue)
    setBusy(true)
    setError('')
    try {
      const ps = effectivePayments(cart, total)
      const change = changeDue(total, ps)
      const r = await sendNow({ kind: 'sale', userId: who.id, body: saleBody(cart, ps) })
      if (r && r.status !== 'applied') return setError(r.note || 'The PC could not take this sale')
      dispatch({ type: 'newSale', saleType: copy.defaultSaleType === 'wholesale' ? 'wholesale' : 'retail' })
      if (!r) return setDone({ text: 'Saved — it will send when the PC is back. No receipt this time.', change })
      setDone({ text: `Sale ${r.receiptNo ?? ''} done. The receipt prints at the counter.`, change, print: copy.askToPrint ? r.saleId : undefined })
      if (!copy.askToPrint && r.saleId) void printSale(r.saleId).catch(() => {})
    } finally {
      setBusy(false)
    }
  }

  if (done) {
    return (
      <div className="flex flex-1 flex-col gap-3 p-4">
        <div role="status" className={cn(CARD, 'rounded-[var(--radius)] p-4 text-[15px] font-semibold')}>{done.text}</div>
        {done.change > 0.005 && (
          <div className="flex justify-between rounded-[var(--radius-sm)] bg-[color-mix(in_srgb,var(--success)_14%,transparent)] px-3 py-3 text-lg font-extrabold text-success">
            <span>Give change</span><span className="tabular-nums">{money(copy.symbol, done.change)}</span>
          </div>
        )}
        <button type="button" onClick={onBack} className="mt-auto rounded-[var(--radius)] bg-brand p-3.5 text-base font-extrabold text-brand-foreground">New sale</button>
        {done.print && <AskPrint saleId={done.print} onClose={() => setDone({ ...done, print: undefined })} />}
      </div>
    )
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto pb-3">
      <div className="flex items-center gap-2 px-3 pb-2 font-bold">
        <button type="button" onClick={onBack} className="flex items-center gap-1"><ChevronLeft size={18} />Back to items</button>
        <span className="ml-auto"><LivePill live={live} /></span>
      </div>
      <CartLines cart={cart} dispatch={dispatch} copy={copy} />
      <div className="mx-3 mt-1 mb-1.5 flex items-baseline justify-between">
        <span className="font-semibold text-muted-foreground">Total</span>
        <span className="font-display text-[30px] font-extrabold tabular-nums">{money(copy.symbol, total)}</span>
      </div>
      <CustomerPick copy={copy} id={cart.customerId} set={(id) => dispatch({ type: 'customer', id })} />
      <PayTenders copy={copy} cart={cart} dispatch={dispatch} live={live} setError={setError} />
      <Owing copy={copy} cart={cart} />
      <p className="mx-3 mt-1.5 flex items-center gap-1.5 text-[11.5px] text-muted-foreground">
        <Printer size={13} />{live ? 'Receipt prints at the counter' : 'PC not reachable — no receipt; the sale sends when it is back'}
      </p>
      {error && <p role="alert" className="mx-3 mt-1.5 text-sm font-semibold text-destructive">{error}</p>}
      <button type="button" disabled={busy || !cart.lines.length} onClick={() => void finish()}
        className="mx-3 mt-3 rounded-[var(--radius)] bg-brand p-3.5 text-base font-extrabold text-brand-foreground disabled:opacity-60">
        {busy ? 'Sending…' : 'Finish sale'}
      </button>
    </div>
  )
}

/** Still to pay, or the change to give, from the payments so far (the till's own sums). */
function Owing({ copy, cart }: { copy: TillCopy; cart: TillState }) {
  const total = saleTotal(cart)
  const ps = effectivePayments(cart, total)
  const left = balanceDue(total, ps)
  const change = changeDue(total, ps)
  if (left > 0.005) {
    return (
      <div role="status" className="mx-3 mt-2 flex justify-between rounded-[var(--radius-sm)] bg-muted px-3 py-2 font-extrabold">
        <span>Still to pay</span><span className="tabular-nums">{money(copy.symbol, left)}</span>
      </div>
    )
  }
  if (change < 0.005) return null
  return (
    <div role="status" className="mx-3 mt-2 flex justify-between rounded-[var(--radius-sm)] bg-[color-mix(in_srgb,var(--success)_14%,transparent)] px-3 py-2 font-extrabold text-success">
      <span>Give change</span><span className="tabular-nums">{money(copy.symbol, change)}</span>
    </div>
  )
}

/** Walk-in, or a customer found by name or phone (credit and cheques need one). */
function CustomerPick({ copy, id, set }: { copy: TillCopy; id: number | null; set: (id: number | null) => void }) {
  const [q, setQ] = useState('')
  const picked = copy.customers.find((c) => c.id === id)
  if (picked) {
    return (
      <div className="mx-3 mb-2 flex items-center gap-2 text-sm">
        <span className="font-semibold">{picked.name}</span><span className="text-muted-foreground">{picked.phone}</span>
        <button type="button" aria-label="Back to walk-in" onClick={() => set(null)} className="ml-auto grid size-7 place-items-center rounded-full bg-muted"><X size={14} /></button>
      </div>
    )
  }
  const s = q.trim().toLowerCase()
  const hits = s ? copy.customers.filter((c) => c.name.toLowerCase().includes(s) || c.phone.includes(s)).slice(0, 5) : []
  return (
    <div className="mx-3 mb-2">
      <input aria-label="Customer" placeholder="Walk-in · find a customer…" autoComplete="off" value={q} onChange={(e) => setQ(e.target.value)} className={cn(FIELD, 'w-full')} />
      {hits.map((c) => (
        <button key={c.id} type="button" onClick={() => { set(c.id); setQ('') }} className="flex w-full justify-between border-b border-border px-2 py-2 text-left text-sm last:border-b-0">
          <span className="font-semibold">{c.name}</span><span className="text-muted-foreground">{c.phone}</span>
        </button>
      ))}
    </div>
  )
}

/** "Ask before printing" (Settings): the question is a dialog, never a toast. */
function AskPrint({ saleId, onClose }: { saleId: number; onClose: () => void }) {
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Print the receipt?</DialogTitle>
          <DialogDescription>It prints at the counter printer.</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Skip</Button>
          <Button onClick={() => { void printSale(saleId).catch(() => {}); onClose() }}>Print</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
