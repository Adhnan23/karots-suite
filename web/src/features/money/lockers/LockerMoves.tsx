import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminLocker, LockerAdjustRequest, ReceiptRef } from '@/api/types.gen'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { moneyDone } from '@/features/purchasing/suppliers/SupplierMoney'
import { errMsg } from '@/features/till/api'
import { dec, money, num } from '@/lib/money'
import { cn } from '@/lib/utils'

const amt = (v: string) => v.replace(/[^\d.,]/g, '')
const tile = 'flex flex-col gap-0.5 rounded-[var(--radius-sm)] border border-border bg-muted px-3 py-2.5 text-left aria-checked:border-selected aria-checked:bg-selected aria-checked:text-selected-foreground'

/** Submit a money move once: the sheet stays disabled while it's in flight, then
 * toasts with "Print receipt" and refreshes every admin list. */
function useMove(onClose: () => void) {
  const qc = useQueryClient()
  const [busy, setBusy] = useState(false)
  const run = async (url: string, json: unknown, msg: string) => {
    if (busy) return
    setBusy(true)
    try {
      moneyDone(msg, await api<ReceiptRef>(url, { json }))
      void qc.invalidateQueries({ queryKey: ['admin'] })
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return { busy, run }
}

function LockerSelect({ label, value, onChange, lockers, sym, not }: {
  label: string; value: number; onChange: (id: number) => void; lockers: AdminLocker[]; sym: string; not?: number
}) {
  return (
    <Field label={label}>
      <select aria-label={label} value={value} onChange={(e) => onChange(Number(e.target.value))} className={cn(INPUT, 'appearance-auto')}>
        <option value={0}>— pick —</option>
        {lockers.filter((l) => l.id !== not).map((l) => <option key={l.id} value={l.id}>{l.name} ({money(sym, l.balance)})</option>)}
      </select>
    </Field>
  )
}

/** What a locker holds after a move; red when it would go below zero. */
function After({ sym, l, delta }: { sym: string; l?: AdminLocker; delta: number }) {
  if (!l || !delta) return null
  const after = num(l.balance) + delta
  return (
    <span className="flex justify-between gap-3 text-[13px]">
      <span className="text-muted-foreground">{l.name} after</span>
      <b className={cn('tabular-nums', after < 0 && 'text-danger')}>{money(sym, after)}{after < 0 && !l.overdraft && ' — not enough in it'}</b>
    </span>
  )
}

/** Move money between two of your lockers (karots-pos LockerTransferForm). */
export function TransferSheet({ lockers, from, sym, onClose }: { lockers: AdminLocker[]; from?: number; sym: string; onClose: () => void }) {
  const [f, setF] = useState({ fromId: from ?? 0, toId: 0, amount: '', note: '' })
  const { busy, run } = useMove(onClose)
  const a = num(f.amount.replace(/,/g, ''))
  const src = lockers.find((l) => l.id === f.fromId)
  const dst = lockers.find((l) => l.id === f.toId)
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title="Move money between lockers" submit="⇄ Move it" busy={busy || !src || !dst || a <= 0}
      onSubmit={() => void run('/admin/lockers/transfer', f, `Moved ${money(sym, a)} from ${src?.name} to ${dst?.name}`)}>
      <div className="grid gap-3 sm:grid-cols-2">
        <LockerSelect label="From" value={f.fromId} onChange={(fromId) => setF({ ...f, fromId, toId: fromId === f.toId ? 0 : f.toId })} lockers={lockers} sym={sym} />
        <LockerSelect label="To" value={f.toId} onChange={(toId) => setF({ ...f, toId })} lockers={lockers} sym={sym} not={f.fromId} />
      </div>
      <Field label="Amount"><input aria-label="Amount" inputMode="decimal" value={f.amount} placeholder="0.00" onChange={(e) => setF({ ...f, amount: amt(e.target.value) })} className={INPUT} /></Field>
      <Field label="Note (optional)"><input aria-label="Note" value={f.note} placeholder="Banked the day's takings" onChange={(e) => setF({ ...f, note: e.target.value })} className={INPUT} /></Field>
      <div className="flex flex-col gap-1">
        <After sym={sym} l={src} delta={-a} />
        <After sym={sym} l={dst} delta={a} />
      </div>
    </FormSheet>
  )
}

const ADJUST: { type: LockerAdjustRequest['type']; label: string; sub: string; dir: 1 | -1 }[] = [
  { type: 'bank_charge', label: '↑ Bank charge', sub: 'Money out · booked as an expense', dir: -1 },
  { type: 'interest', label: '↓ Interest', sub: 'Money in · other income', dir: 1 },
  { type: 'adjust_up', label: '↓ Count is higher', sub: 'Correct the balance up', dir: 1 },
  { type: 'adjust_down', label: '↑ Count is lower', sub: 'Correct the balance down', dir: -1 },
]

/** Bank charge, interest, or a count correction on one locker (karots-pos LockerAdjustForm). */
export function AdjustSheet({ l, sym, onClose }: { l: AdminLocker; sym: string; onClose: () => void }) {
  const [f, setF] = useState<LockerAdjustRequest>({ type: l.kind === 'bank' ? 'bank_charge' : 'adjust_up', amount: '', note: '' })
  const { busy, run } = useMove(onClose)
  const a = num(f.amount.replace(/,/g, ''))
  const opt = ADJUST.find((o) => o.type === f.type)!
  const needNote = f.type.startsWith('adjust')
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Adjust ${l.name}`} submit="Record it" busy={busy || a <= 0 || (needNote && !f.note.trim())}
      onSubmit={() => void run(`/admin/lockers/${l.id}/adjust`, f, `${opt.label.slice(2)} of ${money(sym, a)} recorded on ${l.name}`)}>
      <div role="radiogroup" aria-label="What happened" className="grid grid-cols-2 gap-1.5">
        {ADJUST.map((o) => (
          <button key={o.type} type="button" role="radio" aria-checked={f.type === o.type} onClick={() => setF({ ...f, type: o.type })} className={tile}>
            <b className={cn('text-sm', f.type !== o.type && (o.dir > 0 ? 'text-success' : 'text-danger'))}>{o.label}</b>
            <small className="text-[12px] opacity-80">{o.sub}</small>
          </button>
        ))}
      </div>
      <Field label="Amount"><input aria-label="Amount" inputMode="decimal" value={f.amount} placeholder="0.00" onChange={(e) => setF({ ...f, amount: amt(e.target.value) })} className={INPUT} /></Field>
      <Field label={needNote ? 'Why' : 'Note (optional)'}>
        <input aria-label="Note" value={f.note} placeholder={needNote ? 'Counted the safe — short by a note' : 'Monthly account fee'} onChange={(e) => setF({ ...f, note: e.target.value })} className={INPUT} />
      </Field>
      <After sym={sym} l={l} delta={a * opt.dir} />
    </FormSheet>
  )
}

/** Bank a card-machine batch: the bank gets it minus the fee (karots-pos SettleCardForm). */
export function SettleSheet({ lockers, sym, onClose }: { lockers: AdminLocker[]; sym: string; onClose: () => void }) {
  const cards = lockers.filter((l) => l.kind === 'card')
  const banks = lockers.filter((l) => l.kind === 'bank')
  const first = cards.length === 1 ? cards[0] : undefined
  const [f, setF] = useState({ cardId: first?.id ?? 0, bankId: banks.length === 1 ? banks[0].id : 0, gross: first && num(first.balance) > 0 ? dec(num(first.balance)) : '', fee: '' })
  const { busy, run } = useMove(onClose)
  const gross = num(f.gross.replace(/,/g, ''))
  const fee = num(f.fee.replace(/,/g, ''))
  const card = cards.find((l) => l.id === f.cardId)
  const bank = banks.find((l) => l.id === f.bankId)
  const bad = fee >= gross && gross > 0
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title="Settle card machine → bank" submit="Settle" busy={busy || !card || !bank || gross <= 0 || bad}
      onSubmit={() => void run('/admin/lockers/settle-card', f, `Settled ${money(sym, gross - fee)} into ${bank?.name}`)}>
      <p className="text-sm text-muted-foreground">When the machine settles a batch, the bank pays in the takings minus its fee.</p>
      <div className="grid gap-3 sm:grid-cols-2">
        <LockerSelect label="Card machine" value={f.cardId} lockers={cards} sym={sym}
          onChange={(cardId) => { const c = cards.find((l) => l.id === cardId); setF({ ...f, cardId, gross: c && num(c.balance) > 0 ? dec(num(c.balance)) : '' }) }} />
        <LockerSelect label="Into bank" value={f.bankId} onChange={(bankId) => setF({ ...f, bankId })} lockers={banks} sym={sym} />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <Field label="Amount settled" hint="The batch total"><input aria-label="Amount settled" inputMode="decimal" value={f.gross} placeholder="0.00" onChange={(e) => setF({ ...f, gross: amt(e.target.value) })} className={INPUT} /></Field>
        <Field label="Service fee" hint="Booked as an expense"><input aria-label="Service fee" inputMode="decimal" value={f.fee} placeholder="0.00" aria-invalid={bad} onChange={(e) => setF({ ...f, fee: amt(e.target.value) })} className={INPUT} /></Field>
      </div>
      <div className="flex items-center justify-between rounded-[var(--radius-sm)] bg-muted px-3 py-2.5">
        <span className="text-sm text-muted-foreground">{bad ? 'The fee must be less than the amount settled' : '↓ Net into the bank'}</span>
        <b className={cn('font-display text-lg tabular-nums', bad ? 'text-danger' : 'text-success')}>{money(sym, Math.max(0, gross - fee))}</b>
      </div>
      <After sym={sym} l={card} delta={-gross} />
    </FormSheet>
  )
}
