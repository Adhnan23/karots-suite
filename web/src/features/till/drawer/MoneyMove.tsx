import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { ReceiptRef } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, drawerQuery, errMsg, holdsCash } from '../api'
import { useTillUi } from '../TillUi'
import { FIELD } from '../ui'
import { lockerDefault } from './count'
import { LockerSelect } from './LockerSelect'

/** Mid-shift cash out (↑ red) or in (↓ green) — karots' money-direction cue. */
export function MoneyMove({ kind }: { kind: 'withdraw' | 'deposit' }) {
  const out = kind === 'withdraw'
  const cfg = useSuspenseQuery(configQuery).data
  const d = useQuery(drawerQuery).data
  const ui = useTillUi()
  const qc = useQueryClient()
  const lockers = (d?.lockers ?? []).filter(holdsCash)
  const [amount, setAmount] = useState('')
  const [reason, setReason] = useState('')
  const [lockerId, setLockerId] = useState(() => lockerDefault(lockers, cfg.defaultLockerId, out ? 'out' : 'in', cfg.allowUntrackedCash))
  const [busy, setBusy] = useState(false)
  const untrackedOk = !out || cfg.allowUntrackedCash

  async function submit() {
    const a = num(amount)
    if (a <= 0) return void toast.error(`Enter an amount to ${kind}`)
    if (out && a > num(d?.summary.expected)) return void toast.error("Can't withdraw more than what's in the drawer")
    if (!untrackedOk && !num(lockerId)) {
      return void toast.error(lockers.length ? 'Choose a locker to withdraw into' : 'No locker set up — ask an admin to add one first')
    }
    setBusy(true)
    try {
      const res = await api<ReceiptRef>(`/till/drawer/${kind}`, {
        json: { amount: String(a), reason, lockerId: num(lockerId), deferDrawerKick: !cfg.askToPrint },
      })
      void qc.invalidateQueries({ queryKey: drawerQuery.queryKey })
      toast.success(out ? 'Cash withdrawn' : 'Cash deposited')
      ui.close()
      ui.afterMove(res.receiptId, true) // auto-print merges the drawer pulse into the slip job
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && ui.close()}>
      <DialogContent className="sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle>
            <span className={out ? 'text-danger' : 'text-success'}>{out ? '↑' : '↓'}</span> {out ? 'Withdraw cash' : 'Deposit cash'}
          </DialogTitle>
          <DialogDescription>In the drawer now: {money(cfg.symbol, d?.summary.expected)}</DialogDescription>
        </DialogHeader>
        <form className="flex flex-col gap-3" onSubmit={(e) => { e.preventDefault(); void submit() }}>
          <input autoFocus aria-label="Amount" placeholder="Amount" inputMode="decimal" autoComplete="off" value={amount}
            onChange={(e) => setAmount(e.target.value.replace(/[^\d.]/g, ''))} className={cn(FIELD, 'font-display text-lg font-bold tabular-nums')} />
          <input aria-label="Reason" placeholder="Reason (optional)" autoComplete="off" value={reason}
            onChange={(e) => setReason(e.target.value)} className={FIELD} />
          <LockerSelect label={out ? 'Put into' : 'Taken from'} lockers={lockers} value={lockerId} onChange={setLockerId}
            untrackedLabel={untrackedOk ? '— untracked (no locker) —' : undefined} sym={cfg.symbol} />
          {!untrackedOk && !lockers.length && <p className="text-sm text-danger">No cash locker set up — ask an admin to add one first.</p>}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={ui.close}>Cancel</Button>
            <Button type="submit" disabled={busy || (!untrackedOk && !lockers.length)}>{out ? 'Withdraw' : 'Deposit'}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
