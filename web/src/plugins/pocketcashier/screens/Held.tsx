import { X } from 'lucide-react'
import { useState } from 'react'
import type { HeldSale, TillCopy } from '@/api/types.gen'
import { toHeldBlob } from '@/features/till/cart/held'
import type { Action as CartAction } from '@/features/till/cart/store'
import { saleTotal } from '@/features/till/cart/totals'
import type { TillState } from '@/features/till/cart/types'
import { FIELD } from '@/features/till/ui'
import { dec, money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { CARD, hhmm } from '../bits'
import { heldList, type HeldRow } from '../drawer'
import type { Who } from '../session'
import { ack, type Action } from '../store/outbox'
import { sendNow } from '../sync'

/** Park the sale on screen, or take one back: this person's held sales from the
 * copy, plus holds made here that haven't reached the PC yet. */
export function Held({ who, copy, out, cart, dispatch, onSent, onResumed }: {
  who: Who
  copy: TillCopy
  out: Action[]
  cart: TillState
  dispatch: (a: CartAction) => void
  onSent: () => void
  onResumed: () => void
}) {
  const [label, setLabel] = useState('')
  const [msg, setMsg] = useState<{ tone: 'ok' | 'bad'; text: string } | null>(null)
  const [busy, setBusy] = useState(false)
  const [asking, setAsking] = useState<string | null>(null) // 'clear' | a held row's key
  const rows = heldList(copy, who.id, out)
  const newSale = () => dispatch({ type: 'newSale', saleType: copy.defaultSaleType === 'wholesale' ? 'wholesale' : 'retail' })

  async function hold() {
    setBusy(true)
    try {
      const body = {
        label: label.trim() || `Held ${hhmm(new Date().toISOString())}`, saleType: cart.saleType, customerId: cart.customerId ?? undefined,
        discount: String(num(cart.billDiscount)), cart: toHeldBlob(cart), itemCount: cart.lines.length, total: dec(saleTotal(cart)),
      }
      const r = await sendNow({ kind: 'hold', userId: who.id, body })
      if (r && r.status !== 'applied') return setMsg({ tone: 'bad', text: r.note || 'The PC could not hold this sale' })
      newSale()
      setLabel('')
      setMsg({ tone: 'ok', text: r ? 'Sale held' : 'Sale held on this phone — it sends when the PC is back' })
      onSent()
    } finally {
      setBusy(false)
    }
  }

  async function resume(h: HeldRow) {
    if (cart.lines.length) return setMsg({ tone: 'bad', text: 'Hold or finish the sale on screen first' })
    setBusy(true)
    try {
      const sale: HeldSale = { ...h, customerId: h.customerId || undefined }
      // ponytail: a local hold taken back while a send of it is under way can still reach
      // the PC; it then shows in Held again (no double sale — it has to be resumed by hand).
      if (h.ref) await ack([h.ref])
      else {
        const r = await sendNow({ kind: 'unhold', userId: who.id, body: { heldId: h.id } })
        if (r?.status === 'skipped') return setMsg({ tone: 'bad', text: 'That sale was already taken back on another till' })
        if (r && r.status !== 'applied') return setMsg({ tone: 'bad', text: r.note || 'The PC could not give this sale back' })
      }
      dispatch({ type: 'resume', held: sale })
      onSent()
      onResumed()
    } finally {
      setBusy(false)
    }
  }

  async function remove(h: HeldRow) {
    setAsking(null)
    setBusy(true)
    try {
      if (h.ref) await ack([h.ref])
      else {
        const r = await sendNow({ kind: 'unhold', userId: who.id, body: { heldId: h.id } })
        if (r && r.status !== 'applied' && r.status !== 'skipped') return setMsg({ tone: 'bad', text: r.note || 'The PC could not remove this held sale' })
      }
      setMsg({ tone: 'ok', text: 'Held sale removed' })
      onSent()
    } finally {
      setBusy(false)
    }
  }
  const key = (h: HeldRow) => h.ref ?? String(h.id)
  const confirm = (yes: string, onYes: () => void) => (
    <span className="flex shrink-0 gap-1.5">
      <button type="button" onClick={() => setAsking(null)} className="rounded-pill bg-muted px-3 py-1.5 text-xs font-bold">Keep</button>
      <button type="button" onClick={onYes} className="rounded-pill bg-destructive px-3 py-1.5 text-xs font-bold text-background">{yes}</button>
    </span>
  )

  return (
    <div className="flex flex-1 flex-col gap-2 overflow-y-auto px-3 pb-3">
      {cart.lines.length > 0 && (
        <div className={cn(CARD, 'flex flex-col gap-2 rounded-[var(--radius)] p-3')}>
          <div className="text-sm font-bold">On screen: {cart.lines.length} {cart.lines.length === 1 ? 'line' : 'lines'} · {money(copy.symbol, saleTotal(cart))}</div>
          <div className="flex gap-2">
            <input aria-label="Name for the held sale" placeholder="Name (optional)" autoComplete="off" value={label} onChange={(e) => setLabel(e.target.value)} className={cn(FIELD, 'min-w-0 flex-1')} />
            <button type="button" disabled={busy} onClick={() => void hold()} className="rounded-pill bg-brand px-4 text-sm font-extrabold text-brand-foreground disabled:opacity-60">Hold</button>
          </div>
          {asking === 'clear' ? (
            <div className="flex items-center gap-2 text-[13px] font-semibold">Every line and payment goes. {confirm('Yes, clear it', () => { setAsking(null); newSale() })}</div>
          ) : (
            <button type="button" onClick={() => setAsking('clear')} className="self-start text-[12.5px] font-semibold text-destructive">Clear sale</button>
          )}
        </div>
      )}
      {msg && <p role={msg.tone === 'bad' ? 'alert' : 'status'} className={cn('px-1 text-sm font-semibold', msg.tone === 'bad' ? 'text-destructive' : 'text-success')}>{msg.text}</p>}
      <ul aria-label="Held sales" className="flex flex-col gap-[7px]">
        {rows.map((h) => (
          <li key={key(h)} className="relative">
            {asking === key(h) && (
              <div className={cn(CARD, 'absolute inset-0 z-10 flex items-center justify-between gap-2 px-3 text-[13px] font-semibold')}>
                Remove “{h.label || 'Held sale'}”? {confirm('Yes, remove', () => void remove(h))}
              </div>
            )}
            <button type="button" disabled={busy} onClick={() => void resume(h)} className={cn(CARD, 'flex w-full items-center gap-2.5 py-2.5 pr-11 pl-3 text-left')}>
              <span className="min-w-0">
                <span className="block truncate text-[13.5px] font-bold">{h.label || 'Held sale'}</span>
                <span className="block text-[11.5px] text-muted-foreground">{h.itemCount} {h.itemCount === 1 ? 'line' : 'lines'} · {hhmm(h.createdAt)}{h.ref ? ' · not sent yet' : ''}</span>
              </span>
              <b className="ml-auto shrink-0 text-[13.5px] tabular-nums">{money(copy.symbol, h.total)}</b>
            </button>
            <button type="button" aria-label={`Remove held sale ${h.label || 'Held sale'}`} onClick={() => setAsking(key(h))}
              className="absolute top-1/2 right-2 grid size-8 -translate-y-1/2 place-items-center rounded-full text-faint"><X size={16} /></button>
          </li>
        ))}
        {!rows.length && <li className="py-8 text-center text-sm text-muted-foreground">No held sales</li>}
      </ul>
    </div>
  )
}
