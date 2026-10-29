import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminCustomerRow, CustomerRequest } from '@/api/types.gen'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { errMsg } from '@/features/till/api'
import { num } from '@/lib/money'

const amt = (v: string) => v.replace(/[^\d.]/g, '')

/** New or edit customer. A phone already on file is
 * refused here (the till reuses it instead). What they owed before the system
 * is asked once, on create. */
export function CustomerForm({ c, onClose, onSaved }: { c?: AdminCustomerRow; onClose: () => void; onSaved?: (r: AdminCustomerRow) => void }) {
  const qc = useQueryClient()
  const [f, setF] = useState<CustomerRequest>({ name: c?.name ?? '', phone: c?.phone ?? '', address: c?.address ?? '', creditLimit: c && num(c.creditLimit) ? c.creditLimit : '', openingBalance: '' })
  const [busy, setBusy] = useState(false)
  const up = (patch: Partial<CustomerRequest>) => setF({ ...f, ...patch })
  async function save() {
    setBusy(true)
    try {
      if (c) await api(`/admin/customers/${c.id}`, { method: 'PUT', json: f })
      const r = c ? { ...c, ...f } : await api<AdminCustomerRow>('/admin/customers', { json: f })
      toast.success(c ? 'Customer updated' : `${r.name} added`)
      void qc.invalidateQueries({ queryKey: ['admin'] })
      void qc.invalidateQueries({ queryKey: ['till'] })
      onClose()
      if (!c) onSaved?.(r)
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={c ? `Edit ${c.name}` : 'New customer'} submit={c ? 'Save' : 'Add customer'}
      busy={busy || f.name.trim().length < 2 || f.phone.trim().length < 4} onSubmit={save}>
      <Field label="Name"><input autoFocus required aria-label="Name" value={f.name} onChange={(e) => up({ name: e.target.value })} className={INPUT} /></Field>
      <Field label="Phone"><input required aria-label="Phone" inputMode="tel" value={f.phone} onChange={(e) => up({ phone: e.target.value })} className={INPUT} /></Field>
      <Field label="Address"><input aria-label="Address" value={f.address} onChange={(e) => up({ address: e.target.value })} className={INPUT} /></Field>
      <div className="grid grid-cols-2 gap-3">
        <Field label="Credit limit" hint="Blank = no credit">
          <input aria-label="Credit limit" inputMode="decimal" value={f.creditLimit} placeholder="0.00" onChange={(e) => up({ creditLimit: amt(e.target.value) })} className={INPUT} />
        </Field>
        {!c && (
          <Field label="Already owes" hint="Before you started with this">
            <input aria-label="Already owes" inputMode="decimal" value={f.openingBalance} placeholder="0.00" onChange={(e) => up({ openingBalance: amt(e.target.value) })} className={INPUT} />
          </Field>
        )}
      </div>
    </FormSheet>
  )
}
