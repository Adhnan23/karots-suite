import { useState } from 'react'
import type { TillCopy } from '@/api/types.gen'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { FIELD } from '@/features/till/ui'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { cashPlaces } from '../drawer'
import type { ActionKind } from '../store/outbox'

export type DrawerKind = Extract<ActionKind, 'open' | 'withdraw' | 'deposit' | 'close'>

const TEXT: Record<DrawerKind, { title: string; amount: string; place: string; untracked: string; go: string }> = {
  open: { title: 'Open the drawer', amount: 'Float in the drawer', place: 'Float taken from', untracked: '— not from a locker —', go: 'Open' },
  withdraw: { title: 'Take cash out', amount: 'Amount', place: 'Put into', untracked: '— not into a locker —', go: 'Take out' },
  deposit: { title: 'Put cash in', amount: 'Amount', place: 'Taken from', untracked: '— not from a locker —', go: 'Put in' },
  close: { title: 'Close the drawer', amount: 'Cash counted in the drawer', place: 'Bank cash into', untracked: '— leave cash in drawer —', go: 'Close' },
}

/** One form for the four drawer moves: an amount, a reason (moves), and the
 * locker the cash comes from or goes to. The phone never pops a drawer. */
export function DrawerSheet({ kind, copy, expected, onSend, onClose }: {
  kind: DrawerKind
  copy: TillCopy
  expected: string
  onSend: (body: Record<string, unknown>) => Promise<string>
  onClose: () => void
}) {
  const t = TEXT[kind]
  const places = cashPlaces(copy)
  // Cash leaving the till must land somewhere unless the shop allows untracked cash.
  const out = kind === 'withdraw' || kind === 'close'
  const untrackedOk = !out || copy.allowUntrackedCash
  const first = places.find((p) => p.id === copy.defaultLockerId) ?? (untrackedOk ? undefined : places[0])
  const [amount, setAmount] = useState(kind === 'close' ? expected : '')
  const [reason, setReason] = useState('')
  const [locker, setLocker] = useState(first ? String(first.id) : '')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function send() {
    const a = num(amount)
    if (kind !== 'open' && kind !== 'close' && a <= 0) return setError('Enter an amount')
    if (!untrackedOk && !num(locker) && a > 0) return setError(places.length ? 'Choose where the cash goes' : 'The shop has no cash locker for this — ask the owner')
    setBusy(true)
    const id = num(locker)
    const body = kind === 'open' ? { openingCash: String(a), sourceLockerId: id }
      : kind === 'close' ? { closingCash: String(a), destLockerId: id }
        : { amount: String(a), reason: reason.trim(), lockerId: id }
    const err = await onSend(body).finally(() => setBusy(false))
    if (err) setError(err)
    else onClose()
  }

  return (
    <Sheet open onOpenChange={(o) => !o && onClose()}>
      <SheetContent side="bottom" className="gap-3 p-4">
        <SheetHeader className="p-0">
          <SheetTitle>{t.title}</SheetTitle>
          {kind !== 'open' && <SheetDescription>The drawer should hold about {money(copy.symbol, expected)}.</SheetDescription>}
        </SheetHeader>
        <label className="flex flex-col gap-1 text-[13px] font-semibold text-muted-foreground">
          {t.amount}
          <input inputMode="decimal" autoComplete="off" value={amount} onChange={(e) => { setAmount(e.target.value.replace(/[^\d.]/g, '')); setError('') }} className={cn(FIELD, 'text-lg tabular-nums')} />
        </label>
        {(kind === 'withdraw' || kind === 'deposit') && (
          <input aria-label="Reason" placeholder="Reason (optional)" autoComplete="off" value={reason} onChange={(e) => setReason(e.target.value)} className={FIELD} />
        )}
        {places.length > 0 && (
          <label className="flex flex-col gap-1 text-[13px] font-semibold text-muted-foreground">
            {t.place}
            <select value={locker} onChange={(e) => { setLocker(e.target.value); setError('') }} className={cn(FIELD, 'appearance-auto')}>
              {untrackedOk && <option value="">{t.untracked}</option>}
              {places.map((p) => <option key={p.id} value={String(p.id)}>{p.name}</option>)}
            </select>
          </label>
        )}
        {error && <p role="alert" className="text-sm font-semibold text-destructive">{error}</p>}
        <button type="button" disabled={busy} onClick={() => void send()} className="rounded-[var(--radius)] bg-brand p-3.5 text-base font-extrabold text-brand-foreground disabled:opacity-60">
          {busy ? 'Sending…' : t.go}
        </button>
      </SheetContent>
    </Sheet>
  )
}
