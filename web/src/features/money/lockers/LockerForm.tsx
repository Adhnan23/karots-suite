import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminLocker, LockerRequest } from '@/api/types.gen'
import { Field, INPUT, Toggle } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'
import { KINDS } from './kinds'

/** New or edit locker (karots-pos LockerForm). The opening balance is asked once,
 * on create; after that the balance only moves through money moves. */
export function LockerForm({ l, onClose }: { l?: AdminLocker; onClose: () => void }) {
  const qc = useQueryClient()
  const [f, setF] = useState<LockerRequest>({
    name: l?.name ?? '', kind: l?.kind ?? 'safe', openingBalance: '', overdraft: l?.overdraft ?? false, cashierAccess: l?.cashierAccess ?? true,
  })
  const [busy, setBusy] = useState(false)
  const up = (patch: Partial<LockerRequest>) => setF({ ...f, ...patch })
  async function save() {
    setBusy(true)
    try {
      if (l) await api(`/admin/lockers/${l.id}`, { method: 'PUT', json: f })
      else await api('/admin/lockers', { json: f })
      toast.success(l ? 'Locker updated' : `${f.name.trim()} added`)
      void qc.invalidateQueries({ queryKey: ['admin'] })
      void qc.invalidateQueries({ queryKey: ['till'] })
      onClose()
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={l ? `Edit ${l.name}` : 'New locker'} submit={l ? 'Save' : 'Add locker'} busy={busy || !f.name.trim()} onSubmit={save}>
      <Field label="Name"><input autoFocus required aria-label="Name" value={f.name} onChange={(e) => up({ name: e.target.value })} className={INPUT} placeholder="Shop safe, BOC account, Owner's pocket" /></Field>
      <fieldset className="flex flex-col gap-1.5">
        <legend className="mb-1.5 text-[13px] font-semibold text-muted-foreground">Kind</legend>
        <div role="radiogroup" aria-label="Kind" className="grid grid-cols-2 gap-1.5 sm:grid-cols-3">
          {KINDS.map(({ kind, label, Icon }) => (
            <button key={kind} type="button" role="radio" aria-checked={f.kind === kind} onClick={() => up({ kind })}
              className={cn('flex items-center gap-2 rounded-[var(--radius-sm)] border border-border bg-muted px-3 py-2.5 text-left text-sm font-semibold',
                'aria-checked:border-selected aria-checked:bg-selected aria-checked:text-selected-foreground')}>
              <Icon size={16} className="flex-none" /> {label}
            </button>
          ))}
        </div>
      </fieldset>
      {!l && (
        <Field label="Money in it now" hint="Counted today. Recorded as the opening balance.">
          <input aria-label="Opening balance" inputMode="decimal" value={f.openingBalance} placeholder="0.00"
            onChange={(e) => up({ openingBalance: e.target.value.replace(f.overdraft ? /[^\d.-]/g : /[^\d.]/g, '') })} className={INPUT} />
        </Field>
      )}
      <Toggle label="Can go below zero" hint="For the owner's pocket — they may put in more than they take." checked={f.overdraft} onChange={(overdraft) => up({ overdraft })} />
      <Toggle label="Cashiers can use it" hint="Move money in or out of it from the till. Off = only you." checked={f.cashierAccess} onChange={(cashierAccess) => up({ cashierAccess })} />
    </FormSheet>
  )
}
