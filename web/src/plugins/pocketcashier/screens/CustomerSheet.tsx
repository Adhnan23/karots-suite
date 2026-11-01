import { ChevronRight, User, X } from 'lucide-react'
import { useState } from 'react'
import type { CopyCustomer, TillCopy, TillCustomer } from '@/api/types.gen'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { balanceText } from '@/features/till/pay/CustomerPicker'
import { FIELD } from '@/features/till/ui'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { CARD } from '../bits'
import { createCustomer, liveCall } from '../quickItem'
import { refresh } from '../sync'

/** The copy's customer as the till's (balanceText reads outstanding). */
const asTill = (c: CopyCustomer): TillCustomer => ({ id: c.id, name: c.name, phone: c.phone, creditLimit: c.creditLimit, outstanding: c.balance })
const line = (sym: string, c: TillCustomer) => `${c.phone} · ${balanceText(sym, c)} · limit ${money(sym, c.creditLimit)}`

/** Who's buying, on Pay: walk-in, or a customer with their balance and limit (the
 * PC's picker). A customer added here (live only) is picked at once. */
export function CustomerRow({ copy, id, live, canManageCredit, set }: { copy: TillCopy; id: number | null; live: boolean; canManageCredit: boolean; set: (id: number | null) => void }) {
  const [open, setOpen] = useState(false)
  const [added, setAdded] = useState<TillCustomer | null>(null)
  const found = copy.customers.find((c) => c.id === id)
  const cur = found ? asTill(found) : added?.id === id ? added : null
  return (
    <>
      <div className={cn(CARD, 'mx-3 mb-2 flex items-center gap-2 px-3 py-2')}>
        <button type="button" onClick={() => setOpen(true)} className="flex min-w-0 flex-1 items-center gap-2.5 text-left">
          <User size={18} className="shrink-0 text-muted-foreground" />
          <span className="min-w-0 flex-1">
            <span className="block truncate text-sm font-bold">{cur ? cur.name : 'Walk-in'}</span>
            <span className="block truncate text-[11.5px] text-muted-foreground">{cur ? line(copy.symbol, cur) : 'Cash sale, no account — tap to pick a customer'}</span>
          </span>
          <ChevronRight size={16} className="shrink-0 text-faint" />
        </button>
        {cur && <button type="button" aria-label="Back to walk-in" onClick={() => set(null)} className="grid size-7 shrink-0 place-items-center rounded-full bg-muted"><X size={14} /></button>}
      </div>
      {cur && canManageCredit && live && <LimitEdit key={cur.id} customer={cur} sym={copy.symbol} onSaved={(limit) => { if (added?.id === cur.id) setAdded({ ...added, creditLimit: limit }); void refresh() }} />}
      {open && <CustomerSheet copy={copy} live={live} onClose={() => setOpen(false)} onPick={(c) => { setAdded(c); set(c.id); setOpen(false); void refresh() }} />}
    </>
  )
}

/** The PC's "Adjust credit limit" (people who manage credit; the PC keeps it). */
function LimitEdit({ customer, sym, onSaved }: { customer: TillCustomer; sym: string; onSaved: (limit: string) => void }) {
  const [limit, setLimit] = useState<string | null>(null)
  const [msg, setMsg] = useState('')
  async function save() {
    if (limit === null || limit.trim() === '') return setMsg('Enter a valid credit limit')
    const r = await liveCall(`/till/customers/${customer.id}/credit-limit`, { creditLimit: limit.trim() }, undefined, 'PATCH')
    if ('error' in r) return setMsg(r.error)
    setLimit(null)
    setMsg(`Credit limit now ${money(sym, limit)}`)
    onSaved(limit.trim())
  }
  return (
    <div className="mx-3 -mt-1 mb-2 flex flex-wrap items-center gap-2 text-[12.5px]">
      {limit === null ? (
        <button type="button" onClick={() => { setMsg(''); setLimit(String(Number(customer.creditLimit) || '')) }} className="font-semibold text-muted-foreground underline-offset-2">Adjust credit limit</button>
      ) : (
        <>
          <input aria-label="New credit limit" inputMode="decimal" autoComplete="off" value={limit} onChange={(e) => setLimit(e.target.value.replace(/[^\d.]/g, ''))} className={cn(FIELD, 'h-8 w-28 tabular-nums')} />
          <button type="button" onClick={() => void save()} className="rounded-pill bg-brand px-3 py-1 font-bold text-brand-foreground">Save</button>
          <button type="button" onClick={() => setLimit(null)} className="font-semibold text-muted-foreground">Cancel</button>
        </>
      )}
      {msg && <span role="status" className="font-semibold text-muted-foreground">{msg}</span>}
    </div>
  )
}

function CustomerSheet({ copy, live, onPick, onClose }: { copy: TillCopy; live: boolean; onPick: (c: TillCustomer) => void; onClose: () => void }) {
  const [q, setQ] = useState('')
  const [adding, setAdding] = useState(false)
  const s = q.trim().toLowerCase()
  const hits = (s ? copy.customers.filter((c) => c.name.toLowerCase().includes(s) || c.phone.includes(s)) : copy.customers).slice(0, 50)
  return (
    <Sheet open onOpenChange={(o) => !o && onClose()}>
      <SheetContent side="bottom" className="max-h-[88dvh] gap-3 p-4">
        <SheetHeader className="p-0">
          <SheetTitle>{adding ? 'Add customer' : 'Customer'}</SheetTitle>
          <SheetDescription>{adding ? 'Phone is required — it stops the same person getting two accounts.' : 'Find by name or phone.'}</SheetDescription>
        </SheetHeader>
        {adding ? <AddForm live={live} onAdded={onPick} onBack={() => setAdding(false)} /> : (
          <>
            <div className="flex gap-2">
              <input aria-label="Find a customer" placeholder="Name or phone…" autoComplete="off" value={q} onChange={(e) => setQ(e.target.value)} className={cn(FIELD, 'min-w-0 flex-1')} />
              <button type="button" onClick={() => setAdding(true)} className="shrink-0 rounded-[var(--radius-sm)] bg-muted px-3 text-sm font-bold">+ New customer</button>
            </div>
            <ul className="-mx-1 min-h-0 overflow-y-auto">
              {hits.map((c) => (
                <li key={c.id}>
                  <button type="button" onClick={() => onPick(asTill(c))} className="flex w-full flex-col border-b border-border px-1 py-2 text-left last:border-b-0">
                    <span className="text-sm font-bold">{c.name}</span>
                    <span className="text-[12px] text-muted-foreground">{line(copy.symbol, asTill(c))}</span>
                  </button>
                </li>
              ))}
              {!hits.length && <li className="py-6 text-center text-sm text-muted-foreground">No customer matches “{q}”</li>}
            </ul>
          </>
        )}
      </SheetContent>
    </Sheet>
  )
}

function AddForm({ live, onAdded, onBack }: { live: boolean; onAdded: (c: TillCustomer) => void; onBack: () => void }) {
  const down = "The PC isn't reachable — add the customer at the counter"
  const [name, setName] = useState('')
  const [phone, setPhone] = useState('')
  const [limit, setLimit] = useState('')
  const [error, setError] = useState(live ? '' : down)
  const [busy, setBusy] = useState(false)
  async function submit() {
    if (!name.trim()) return setError('Enter a customer name')
    if (!phone.trim()) return setError('Enter a phone number')
    setBusy(true)
    const r = await createCustomer({ name: name.trim(), phone: phone.trim(), creditLimit: limit || '0' })
    setBusy(false)
    if ('error' in r) setError(r.error)
    else onAdded(r.customer)
  }
  return (
    <form className="flex flex-col gap-2.5" autoComplete="off" onSubmit={(e) => { e.preventDefault(); void submit() }}>
      <input aria-label="Customer name" placeholder="Name" value={name} onChange={(e) => setName(e.target.value)} className={FIELD} />
      <input aria-label="Customer phone" placeholder="Phone" inputMode="tel" value={phone} onChange={(e) => setPhone(e.target.value)} className={FIELD} />
      <input aria-label="Credit limit" placeholder="Credit limit (optional)" inputMode="decimal" value={limit}
        onChange={(e) => setLimit(e.target.value.replace(/[^\d.]/g, ''))} className={cn(FIELD, 'tabular-nums')} />
      {error && <p role="alert" className="text-sm font-semibold text-destructive">{error}</p>}
      <div className="flex gap-2">
        <button type="button" onClick={onBack} className="flex-1 rounded-[var(--radius)] bg-muted p-3 text-sm font-bold">Back</button>
        <button type="submit" disabled={busy || !live} className="flex-1 rounded-[var(--radius)] bg-brand p-3 text-sm font-extrabold text-brand-foreground disabled:opacity-60">
          {busy ? 'Adding…' : 'Add customer'}
        </button>
      </div>
    </form>
  )
}
