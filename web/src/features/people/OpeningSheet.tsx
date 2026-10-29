import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { OpeningRequest } from '@/api/types.gen'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { errMsg } from '@/features/till/api'
import { money, num, r2 } from '@/lib/money'
import { cn } from '@/lib/utils'

/** Correct the old (pre-system) balance of a customer or supplier. The part backed by bills, payments and returns is locked;
 * only the old part moves. Negative = they were in credit before go-live. */
export function OpeningSheet({ kind, id, name, owed, oldDebt, sym, onClose }: {
  kind: 'supplier' | 'customer'; id: number; name: string; owed: string; oldDebt: string; sym: string; onClose: () => void
}) {
  const qc = useQueryClient()
  const [opening, setOpening] = useState(r2(num(oldDebt)).toFixed(2))
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const locked = r2(num(owed) - num(oldDebt))
  const next = r2(locked + num(opening))
  const who = kind === 'supplier' ? 'we owe them' : 'they owe us'
  async function save() {
    setBusy(true)
    try {
      await api(`/admin/${kind}s/${id}/opening`, { json: { opening, note } satisfies OpeningRequest })
      toast.success('Old balance corrected')
      void qc.invalidateQueries({ queryKey: ['admin'] })
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  const box = 'rounded-[var(--radius-sm)] bg-muted px-3 py-2.5'
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Old balance · ${name}`} submit="Save" busy={busy || opening === '' || num(opening) === num(oldDebt)} onSubmit={save}>
      <div className="grid grid-cols-2 gap-2 text-sm">
        <div className={box}><small className="text-muted-foreground">Total ({who})</small><b className="block tabular-nums">{money(sym, owed)}</b></div>
        <div className={box}><small className="text-muted-foreground">From bills &amp; payments · locked</small><b className="block tabular-nums">{money(sym, locked)}</b></div>
      </div>
      <Field label="Old balance (before the system)" hint="Negative if they were in credit before you started.">
        <input autoFocus aria-label="Old balance" inputMode="decimal" value={opening} onChange={(e) => setOpening(e.target.value.replace(/[^\d.-]/g, ''))}
          className={cn(INPUT, 'h-[52px] font-display text-2xl font-bold tabular-nums')} />
      </Field>
      <p className="text-sm text-muted-foreground">New total <b className="text-foreground tabular-nums">{money(sym, next)}</b></p>
      <Field label="Why"><input aria-label="Why" value={note} onChange={(e) => setNote(e.target.value)} className={INPUT} placeholder="e.g. found an old bill (kept in the audit log)" /></Field>
    </FormSheet>
  )
}
