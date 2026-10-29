import { queryOptions, useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import { meQuery } from '@/api/session'
import type { PresenceRow, StaffRequest, StaffUser } from '@/api/types.gen'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { Field, INPUT, Toggle } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'
import { placeOf, presenceQuery, StateDot, stateText } from './presence'

export const usersQuery = queryOptions({ queryKey: ['admin', 'users'], queryFn: () => api<StaffUser[]>('/admin/users') })

type Role = 'admin' | 'manager' | 'cashier'
const ROLES: { v: Role; label: string; what: string }[] = [
  { v: 'cashier', label: 'Cashier', what: 'Sells at the till. Everything else only with the switches below.' },
  { v: 'manager', label: 'Manager', what: 'Till plus the back office — stock, purchasing, customers.' },
  { v: 'admin', label: 'Admin', what: 'Everything, including staff logins and settings.' },
]
const PERMS: { k: 'canHandleSuppliers' | 'canManageCredit' | 'canSeeCost' | 'canSeeLowStock'; label: string; hint: string }[] = [
  { k: 'canHandleSuppliers', label: 'Counter operations', hint: 'Pay suppliers, take in deliveries, place orders and record expenses at the till. They will see cost prices.' },
  { k: 'canManageCredit', label: 'Manage credit at the till', hint: "Approve a credit sale over a customer's limit and raise a limit." },
  { k: 'canSeeCost', label: 'See product cost at the till', hint: "The ⓘ on a till card shows cost and margin — handy for deciding a discount." },
  { k: 'canSeeLowStock', label: 'See low stock at the till', hint: 'A Low stock tab with what to order and from whom. To place the orders as well they need the first permission above too.' },
]

/** Users: who can sign in, what they are
 * and what extra they may do. The hidden system user never appears here. */
export function UsersPage() {
  const us = useQuery(usersQuery).data
  const who = useQuery(presenceQuery).data
  const me = useSuspenseQuery(meQuery).data
  const [editing, setEditing] = useState<StaffUser | 'new' | null>(null)
  return (
    <>
      <PageHead title="Users" sub="Who can sign in, and what each person may do."
        actions={<Button className="h-10 gap-1.5 px-4 font-bold" onClick={() => setEditing('new')}><Plus size={16} /> New user</Button>} />
      {!us ? <EmptyState title="Loading…" /> : (
        <ul aria-label="Users" className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {us.map((u) => (
            <li key={u.id}>
              <button type="button" onClick={() => setEditing(u)} className={cn(SURFACE, 'flex w-full items-start gap-3 p-4 text-left hover:border-selected', !u.active && 'opacity-60')}>
                <span className="grid size-11 flex-none place-items-center rounded-full bg-brand-soft font-display text-lg font-bold">{u.name.slice(0, 1).toUpperCase()}</span>
                <span className="min-w-0 flex-1">
                  <span className="flex flex-wrap items-center gap-1.5">
                    <b className="truncate">{u.name}</b>
                    <Tag tone={u.role === 'admin' ? 'brand' : 'muted'}>{u.role}</Tag>
                    {!u.active && <Tag tone="danger">deactivated</Tag>}
                    {u.id === me.user.id && <Tag tone="muted">you</Tag>}
                  </span>
                  <small className="block text-faint tabular-nums">{u.phone}{u.mustChangePin && ' · must set a new PIN'}</small>
                  <Live r={who?.find((r) => r.userId === u.id)} />
                  <span className="mt-2 flex flex-wrap gap-1">
                    {PERMS.filter((p) => u[p.k]).map((p) => <span key={p.k} className="rounded-pill bg-muted px-2 py-0.5 text-[11px] font-semibold text-muted-foreground">{p.label}</span>)}
                    {u.receiptPrinter && <span className="rounded-pill bg-muted px-2 py-0.5 text-[11px] font-semibold text-muted-foreground">own printer</span>}
                  </span>
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
      {editing && <UserForm u={editing === 'new' ? undefined : editing} self={editing !== 'new' && editing.id === me.user.id} onClose={() => setEditing(null)} />}
    </>
  )
}

/** Where this user is right now (their freshest device), or nothing when never seen. */
function Live({ r }: { r?: PresenceRow }) {
  if (!r) return null
  const on = r.state !== 'away' && r.state !== 'signedOut'
  return <span className="mt-1 flex items-center gap-2 text-[13px]"><StateDot state={r.state} />{stateText(r)}{on && ` · ${r.note || placeOf(r.path)}`}</span>
}

function UserForm({ u, self, onClose }: { u?: StaffUser; self: boolean; onClose: () => void }) {
  const qc = useQueryClient()
  const [f, setF] = useState<StaffRequest>({ name: u?.name ?? '', phone: u?.phone ?? '', role: u?.role ?? 'cashier', pin: '', receiptPrinter: u?.receiptPrinter ?? '',
    canHandleSuppliers: u?.canHandleSuppliers ?? false, canManageCredit: u?.canManageCredit ?? false, canSeeCost: u?.canSeeCost ?? false, canSeeLowStock: u?.canSeeLowStock ?? false })
  const [busy, setBusy] = useState(false)
  const up = (patch: Partial<StaffRequest>) => setF({ ...f, ...patch })
  const done = (msg: string) => { toast.success(msg); void qc.invalidateQueries({ queryKey: ['admin', 'users'] }); onClose() }
  async function save() {
    setBusy(true)
    try {
      if (u) await api(`/admin/users/${u.id}`, { method: 'PUT', json: f })
      else await api('/admin/users', { json: f })
      done(u ? `${f.name} updated${f.pin ? ' · new PIN set' : ''}` : `${f.name} can now sign in`)
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  const toggle = async () => {
    await api(`/admin/users/${u!.id}${u!.active ? '' : '/activate'}`, { method: u!.active ? 'DELETE' : 'POST' })
    done(u!.active ? `${u!.name} can no longer sign in` : `${u!.name} can sign in again`)
  }
  const pinOk = u ? f.pin === '' || /^\d{4,6}$/.test(f.pin) : /^\d{4,6}$/.test(f.pin)
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={u ? `Edit ${u.name}` : 'New user'} submit={u ? 'Save' : 'Add user'} wide
      busy={busy || f.name.trim().length < 2 || f.phone.trim().length < 4 || !pinOk} onSubmit={save}
      extra={u && !self && (u.active
        ? <ConfirmButton label="Deactivate" title={`Deactivate ${u.name}?`} body="They can't sign in any more. Their sales and history stay." confirm="Deactivate" onConfirm={toggle} />
        : <Button type="button" variant="outline" className="h-10 px-4" onClick={() => void toggle().catch((e) => toast.error(errMsg(e)))}>Reactivate</Button>)}>
      <div className="grid grid-cols-2 gap-3">
        <Field label="Name"><input autoFocus aria-label="Name" value={f.name} onChange={(e) => up({ name: e.target.value })} className={INPUT} /></Field>
        <Field label="Phone" hint="They sign in with this and their PIN."><input aria-label="Phone" inputMode="tel" autoComplete="off" placeholder="07XXXXXXXX" value={f.phone} onChange={(e) => up({ phone: e.target.value })} className={INPUT} /></Field>
      </div>
      <fieldset className="flex flex-col gap-1.5">
        <legend className="mb-1.5 text-[13px] font-semibold text-muted-foreground">Role</legend>
        {ROLES.map((r) => (
          <label key={r.v} className="flex cursor-pointer items-center gap-3 rounded-[var(--radius-sm)] border border-border px-3 py-2 has-checked:border-selected has-checked:bg-muted">
            <input type="radio" name="ks-role" checked={f.role === r.v} disabled={self && r.v !== 'admin'} onChange={() => up({ role: r.v })} />
            <span className="text-sm"><b>{r.label}</b><small className="block text-faint">{r.what}</small></span>
          </label>
        ))}
      </fieldset>
      <Field label={u ? 'New PIN' : 'PIN'} hint={u ? 'Leave blank to keep theirs. A new PIN signs them out everywhere.' : '4 to 6 digits.'}>
        {/* type="text" + text-security, never type="password": a shared counter's PIN must not land in a password manager. */}
        <input aria-label="PIN" name="ks-npn" type="text" inputMode="numeric" autoComplete="off" maxLength={6} value={f.pin} onChange={(e) => up({ pin: e.target.value.replace(/\D/g, '') })}
          style={{ WebkitTextSecurity: 'disc' } as React.CSSProperties} className={cn(INPUT, 'tracking-[0.3em]')} placeholder={u ? 'unchanged' : '••••'} />
      </Field>
      {f.role === 'cashier' && PERMS.map((p) => <Toggle key={p.k} label={p.label} hint={p.hint} checked={f[p.k]} onChange={(v) => up({ [p.k]: v })} />)}
      <Field label="Receipt printer (this person's counter)" hint="Blank = the shop's printer. For a second counter, e.g. tcp://192.168.1.50:9100.">
        <input aria-label="Receipt printer" value={f.receiptPrinter} onChange={(e) => up({ receiptPrinter: e.target.value })} className={INPUT} placeholder="Shop default" />
      </Field>
    </FormSheet>
  )
}
