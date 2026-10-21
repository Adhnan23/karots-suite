import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminExpenseRequest, AdminExpenseRow, AdminExpenseSummary, ReceiptRef } from '@/api/types.gen'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { Button } from '@/components/ui/button'
import { moneySourcesQuery } from '@/features/purchasing/MoneySource'
import { moneyDone } from '@/features/purchasing/suppliers/SupplierMoney'
import { errMsg } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { expenseVoidQuery } from '../queries'

const today = () => new Date().toLocaleDateString('en-CA')
const amt = (v: string) => v.replace(/[^\d.,]/g, '')

/** Every place of ours money can leave or come back to, or outside the shop ("").
 * Never a card-machine account: its money leaves only through Settle card. */
export function PlacePicker({ label, value, onChange, sym, outside }: { label: string; value: string; onChange: (v: string) => void; sym: string; outside?: string }) {
  const all = (useQuery(moneySourcesQuery).data ?? []).filter((s) => s.kind !== 'card')
  return (
    <select aria-label={label} value={value} onChange={(e) => onChange(e.target.value)} className={cn(INPUT, 'appearance-auto')}>
      <option value="">{outside ?? '— pick —'}</option>
      {['Lockers', 'Tills'].map((g) => all.some((s) => s.group === g) && (
        <optgroup key={g} label={g}>
          {all.filter((s) => s.group === g).map((s) => <option key={s.value} value={s.value}>{s.label}{s.group === 'Lockers' ? ` (${money(sym, s.balance)})` : ''}</option>)}
        </optgroup>
      ))}
    </select>
  )
}

/** Record or edit an expense (karots-pos ExpenseForm). Once money has left a
 * place of ours for it, the amount and where it came from are fixed: void it
 * and record it again. */
export function ExpenseForm({ e, meta, sym, onClose, onVoid }: { e?: AdminExpenseRow; meta?: AdminExpenseSummary; sym: string; onClose: () => void; onVoid?: () => void }) {
  const qc = useQueryClient()
  const [f, setF] = useState<AdminExpenseRequest>({
    category: e?.category ?? '', amount: e ? e.amount : '', description: e?.description ?? '', date: e?.date ?? today(), serviceId: e?.serviceId ?? 0, source: '',
  })
  const [busy, setBusy] = useState(false)
  const up = (p: Partial<AdminExpenseRequest>) => setF({ ...f, ...p })
  const locked = !!e?.paidFrom
  const a = num(f.amount.replace(/,/g, ''))
  async function save() {
    if (busy) return
    setBusy(true)
    try {
      if (e) {
        await api(`/admin/expenses/${e.id}`, { method: 'PUT', json: f })
        toast.success('Expense updated')
      } else moneyDone(`${f.category} · ${money(sym, a)} recorded`, await api<ReceiptRef>('/admin/expenses', { json: f }))
      void qc.invalidateQueries({ queryKey: ['admin'] })
      onClose()
    } catch (err) {
      toast.error(errMsg(err))
    } finally {
      setBusy(false)
    }
  }
  const quick = (meta?.categories ?? []).slice(0, 8)
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={e ? 'Edit expense' : 'Record an expense'} submit={e ? 'Save' : '↑ Record it'}
      busy={busy || !f.category.trim() || a <= 0} onSubmit={save}
      extra={e && onVoid && <Button type="button" variant="ghost" className="h-10 px-3 text-danger" onClick={onVoid}>Void…</Button>}>
      <Field label="Category">
        <div className="flex flex-wrap gap-1.5">
          {quick.map((c) => (
            <button key={c} type="button" aria-pressed={f.category === c} onClick={() => up({ category: c })}
              className="rounded-pill border border-border bg-muted px-3 py-1.5 text-[13px] font-semibold text-muted-foreground aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground">{c}</button>
          ))}
        </div>
        <input aria-label="Category" list="admin-expense-categories" placeholder="…or type one" value={f.category} onChange={(x) => up({ category: x.target.value })} className={INPUT} />
        <datalist id="admin-expense-categories">{meta?.categories.map((c) => <option key={c} value={c} />)}</datalist>
      </Field>
      <div className="grid grid-cols-2 gap-3">
        <Field label="Amount" hint={locked ? `Paid from ${e?.paidFrom} — to change it, void and record again` : undefined}>
          <input aria-label="Amount" inputMode="decimal" readOnly={locked} value={f.amount} placeholder="0.00" onChange={(x) => up({ amount: amt(x.target.value) })} className={cn(INPUT, locked && 'opacity-70')} />
        </Field>
        <Field label="Date"><input type="date" aria-label="Date" value={f.date} max={today()} onChange={(x) => up({ date: x.target.value })} className={INPUT} /></Field>
      </div>
      {!e ? (
        <Field label="↑ Paid from" hint={f.source ? 'The money leaves here — a receipt is recorded.' : 'Outside the shop: nothing leaves a locker or till here.'}>
          <PlacePicker label="Paid from" value={f.source} onChange={(source) => up({ source })} sym={sym} outside="Outside the shop (owner paid, bank card…)" />
        </Field>
      ) : (
        <p className="rounded-[var(--radius-sm)] bg-muted px-3 py-2 text-[13px] text-muted-foreground">Paid from <b className="text-foreground">{e.paidFrom || 'outside the shop'}</b></p>
      )}
      <Field label="Note (optional)"><input aria-label="Note" value={f.description} placeholder="What it was for" onChange={(x) => up({ description: x.target.value })} className={INPUT} /></Field>
      {!!meta?.services.length && (
        <Field label="For a service (optional)" hint="Shows what running a service costs. Counted once, as an expense.">
          <select aria-label="For a service" value={f.serviceId} onChange={(x) => up({ serviceId: Number(x.target.value) })} className={cn(INPUT, 'appearance-auto')}>
            <option value={0}>— none —</option>
            {meta.services.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
          </select>
        </Field>
      )}
    </FormSheet>
  )
}

/** Void an expense (karots-pos d66ac18): money that left a place of ours goes
 * back into one — pre-picked: the locker it came out of. */
export function VoidSheet({ e, sym, onClose }: { e: AdminExpenseRow; sym: string; onClose: () => void }) {
  const qc = useQueryClient()
  const info = useQuery(expenseVoidQuery(e.id)).data
  const [dest, setDest] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const to = dest ?? info?.source ?? ''
  async function go() {
    if (busy) return
    setBusy(true)
    try {
      moneyDone(info?.paid ? `Voided — ${money(sym, e.amount)} put back` : 'Expense voided', await api<ReceiptRef>(`/admin/expenses/${e.id}/void`, { json: { dest: to } }))
      void qc.invalidateQueries({ queryKey: ['admin'] })
      onClose()
    } catch (err) {
      toast.error(errMsg(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Void ${e.category} · ${money(sym, e.amount)}?`} submit="Void expense"
      busy={busy || !info || !!info.owner || (info.paid && !to)} onSubmit={go}>
      {!info ? <p className="text-sm text-muted-foreground">Checking…</p> : info.owner ? (
        <p className="rounded-[var(--radius-sm)] bg-[color-mix(in_srgb,var(--warning)_14%,transparent)] px-3 py-2 text-[13px] font-semibold text-warning">
          This expense belongs to {info.owner}. Undo it there instead.
        </p>
      ) : info.paid ? (
        <>
          <p className="text-sm text-muted-foreground">It was paid from <b className="text-foreground">{info.from}</b>. The money goes back and the expense is removed.</p>
          <Field label={`↓ Put ${money(sym, e.amount)} back into`}>
            <PlacePicker label="Put it back into" value={to} onChange={setDest} sym={sym} />
          </Field>
        </>
      ) : (
        <p className="text-sm text-muted-foreground">It was paid from outside the shop, so no money moves — the expense is just removed.</p>
      )}
    </FormSheet>
  )
}
