import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { PickerShell } from '@/components/admin/PickerShell'
import { CategoryPicker } from '@/features/admin/categories/CategoryPicker'
import { supplierOptionsQuery } from '@/features/admin/products/queries'
import { unitsQuery } from '@/features/admin/units/UnitsPage'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'
import { decline, listQuery, makeProduct, merge, updateNote } from './api'
import { askLabel } from './labels'
import type { RequestRow } from './types'

type SheetProps = { r: RequestRow; onClose: () => void }

async function run(setBusy: (b: boolean) => void, p: () => Promise<unknown>, done: string, onClose: () => void) {
  setBusy(true)
  try {
    await p()
    toast.success(done)
    onClose()
  } catch (e) {
    toast.error(errMsg(e))
  } finally {
    setBusy(false)
  }
}

const money = (s: string) => s.replace(/[^\d.]/g, '')

/** Make a product from the request (karots-pos's convert, plus cost and supplier). */
export function ProductSheet({ r, onClose }: SheetProps) {
  const units = useQuery(unitsQuery).data ?? []
  const suppliers = useQuery(supplierOptionsQuery).data ?? []
  const [f, setF] = useState({ name: r.note, categoryId: 0, unitId: 0, sellingPrice: '', costPrice: '', supplierId: 0 })
  const [busy, setBusy] = useState(false)
  const unitId = f.unitId || units.find((u) => u.abbreviation === 'pcs')?.id || units[0]?.id || 0
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title="Make a product" submit="Create product" busy={busy}
      onSubmit={() => {
        if (!f.categoryId) return void toast.error('Pick a category')
        return run(setBusy, () => makeProduct(r.id, { ...f, unitId }), `${f.name.trim()} added — it starts with no stock`, onClose)
      }}>
      <Field label="Product name" hint="Tidy up the customer's wording into a real product name.">
        <input required autoFocus aria-label="Product name" value={f.name} onChange={(e) => setF({ ...f, name: e.target.value })} className={INPUT} />
      </Field>
      <div className="grid grid-cols-2 gap-3">
        <Field label="Category"><CategoryPicker label="Category" allowCreate value={f.categoryId} onChange={(categoryId) => setF({ ...f, categoryId })} /></Field>
        <Field label="Unit">
          <select aria-label="Unit" value={unitId} onChange={(e) => setF({ ...f, unitId: Number(e.target.value) })} className={cn(INPUT, 'appearance-auto')}>
            {units.map((u) => <option key={u.id} value={u.id}>{u.name} ({u.abbreviation})</option>)}
          </select>
        </Field>
        <Field label="Selling price" hint="Optional">
          <input inputMode="decimal" aria-label="Selling price" placeholder="0.00" value={f.sellingPrice}
            onChange={(e) => setF({ ...f, sellingPrice: money(e.target.value) })} className={cn(INPUT, 'tabular-nums')} />
        </Field>
        <Field label="Cost" hint="Optional">
          <input inputMode="decimal" aria-label="Cost" placeholder="0.00" value={f.costPrice}
            onChange={(e) => setF({ ...f, costPrice: money(e.target.value) })} className={cn(INPUT, 'tabular-nums')} />
        </Field>
      </div>
      <Field label="Supplier" hint="Starts with no stock — add it through Stock → Intake or a purchase.">
        <select aria-label="Supplier" value={f.supplierId} onChange={(e) => setF({ ...f, supplierId: Number(e.target.value) })} className={cn(INPUT, 'appearance-auto')}>
          <option value={0}>No preferred supplier</option>
          {suppliers.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
        </select>
      </Field>
    </FormSheet>
  )
}

/** Won't stock: the reason is what the till shows the next customer who asks. */
export function DeclineSheet({ r, onClose }: SheetProps) {
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title="Won't stock" submit="Save" busy={busy}
      onSubmit={() => run(setBusy, () => decline(r.id, reason.trim()), `“${r.note}” marked won't stock`, onClose)}>
      <p className="text-sm text-muted-foreground">{r.note}</p>
      <Field label="Why not?" hint="Cashiers see this when the next customer asks for it.">
        <input required autoFocus aria-label="Why not?" placeholder="e.g. discontinued, too dear to bring in" value={reason}
          onChange={(e) => setReason(e.target.value)} className={INPUT} />
      </Field>
    </FormSheet>
  )
}

export function EditSheet({ r, onClose }: SheetProps) {
  const [note, setNote] = useState(r.note)
  const [busy, setBusy] = useState(false)
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title="Edit request" submit="Save" busy={busy}
      onSubmit={() => run(setBusy, () => updateNote(r.id, note.trim()), 'Request saved', onClose)}>
      <Field label="What they asked for">
        <textarea required autoFocus rows={3} aria-label="What they asked for" value={note} onChange={(e) => setNote(e.target.value)} className={cn(INPUT, 'h-auto py-2')} />
      </Field>
    </FormSheet>
  )
}

/** Merge into another open request: every ask moves over, this one goes. */
export function MergePicker({ r, onClose }: SheetProps) {
  const [q, setQ] = useState('')
  const [busy, setBusy] = useState(false)
  const rows = (useQuery(listQuery('open', q, 1)).data?.rows ?? []).filter((x) => x.id !== r.id && x.status === 'open')
  return (
    <PickerShell open onOpenChange={(o) => !o && onClose()} title={`Merge “${r.note}” into…`}>
      <input autoFocus aria-label="Search requests" placeholder="Search open requests" value={q} onChange={(e) => setQ(e.target.value)} className={INPUT} />
      <div className="-mx-1 flex-1 overflow-y-auto">
        {rows.length === 0 && <p className="px-1 py-6 text-center text-sm text-muted-foreground">No other open request matches.</p>}
        {rows.map((x) => (
          <button key={x.id} type="button" disabled={busy}
            onClick={() => run(setBusy, () => merge(r.id, x.id), `Merged into “${x.note}”`, onClose)}
            className="flex w-full flex-col items-start rounded-[var(--radius-sm)] px-3 py-2.5 text-left hover:bg-muted">
            <span className="font-semibold">{x.note}</span>
            <span className="text-xs text-muted-foreground">{askLabel(x.askCount, x.lastAsked)}</span>
          </button>
        ))}
      </div>
    </PickerShell>
  )
}
