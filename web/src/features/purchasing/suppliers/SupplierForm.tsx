import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminSupplierRow, SupplierRequest } from '@/api/types.gen'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { errMsg } from '@/features/till/api'

/** New or edit supplier. The opening — what was already
 * owed before the shop started using this — is asked once, on create. */
export function SupplierForm({ s, onClose, onSaved }: { s?: AdminSupplierRow; onClose: () => void; onSaved?: (r: AdminSupplierRow) => void }) {
  const qc = useQueryClient()
  const [f, setF] = useState<SupplierRequest>({
    name: s?.name ?? '', contact: s?.contact ?? '', phone: s?.phone ?? '', address: s?.address ?? '', creditDays: s?.creditDays ?? 0, openingBalance: '',
  })
  const [busy, setBusy] = useState(false)
  const up = (patch: Partial<SupplierRequest>) => setF({ ...f, ...patch })
  async function save() {
    setBusy(true)
    try {
      if (s) await api(`/admin/suppliers/${s.id}`, { method: 'PUT', json: f })
      const r = s ? { ...s, ...f } : await api<AdminSupplierRow>('/admin/suppliers', { json: f })
      toast.success(s ? 'Supplier updated' : `${r.name} added`)
      void qc.invalidateQueries({ queryKey: ['admin'] })
      void qc.invalidateQueries({ queryKey: ['till'] })
      onClose()
      if (!s) onSaved?.(r)
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={s ? `Edit ${s.name}` : 'New supplier'} submit={s ? 'Save' : 'Add supplier'} busy={busy || f.name.trim().length < 2} onSubmit={save}>
      <Field label="Name"><input autoFocus required aria-label="Name" value={f.name} onChange={(e) => up({ name: e.target.value })} className={INPUT} placeholder="Company or person" /></Field>
      <div className="grid grid-cols-2 gap-3">
        <Field label="Contact person"><input aria-label="Contact person" value={f.contact} onChange={(e) => up({ contact: e.target.value })} className={INPUT} /></Field>
        <Field label="Phone"><input aria-label="Phone" inputMode="tel" value={f.phone} onChange={(e) => up({ phone: e.target.value })} className={INPUT} /></Field>
      </div>
      <Field label="Address"><input aria-label="Address" value={f.address} onChange={(e) => up({ address: e.target.value })} className={INPUT} /></Field>
      <div className="grid grid-cols-2 gap-3">
        <Field label="Credit days" hint="0 = pay on delivery">
          <input aria-label="Credit days" inputMode="numeric" value={f.creditDays || ''} placeholder="0" onChange={(e) => up({ creditDays: Number(e.target.value.replace(/\D/g, '')) || 0 })} className={INPUT} />
        </Field>
        {!s && (
          <Field label="Already owed" hint="Before you started with this">
            <input aria-label="Already owed" inputMode="decimal" value={f.openingBalance} placeholder="0.00" onChange={(e) => up({ openingBalance: e.target.value.replace(/[^\d.]/g, '') })} className={INPUT} />
          </Field>
        )}
      </div>
    </FormSheet>
  )
}
