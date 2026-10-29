import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'
import { Archive, ArrowLeftRight, CreditCard, MoreHorizontal, Pencil, Plus, RotateCcw, SlidersHorizontal } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { AdminLocker, TillCashRequest } from '@/api/types.gen'
import { Field, INPUT, Toggle } from '@/components/admin/fields'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Tag } from '@/features/admin/products/ProductsPage'
import { configQuery, errMsg, holdsCash } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { lockersQuery } from '../queries'
import { kindOf } from './kinds'
import { LockerForm } from './LockerForm'
import { AdjustSheet, SettleSheet, TransferSheet } from './LockerMoves'

const route = getRouteApi('/_authed/admin/lockers/')

/** A balance; red below zero. */
export function Balance({ sym, v, className }: { sym: string; v: string; className?: string }) {
  return <b className={cn('tabular-nums', num(v) < 0 && 'text-danger', className)}>{money(sym, v)}</b>
}

/** Lockers (karots-pos /admin/lockers): money kept outside the tills, each with
 * its live balance, plus how the till's cash dialogs use them. */
export function LockersPage() {
  const disabled = !!route.useSearch().disabled
  const navigate = route.useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const data = useQuery(lockersQuery(disabled)).data
  const [editing, setEditing] = useState<AdminLocker | 'new' | null>(null)
  const [archiving, setArchiving] = useState<AdminLocker | null>(null)
  const [move, setMove] = useState<{ kind: 'transfer' | 'settle' | 'adjust'; l?: AdminLocker } | null>(null)
  const qc = useQueryClient()
  const setActive = async (l: AdminLocker, on: boolean) => {
    await api(`/admin/lockers/${l.id}${on ? '/activate' : ''}`, { method: on ? 'POST' : 'DELETE' })
    toast.success(on ? `${l.name} is back in use` : `${l.name} archived`)
    void qc.invalidateQueries({ queryKey: ['admin'] })
  }
  // Cheques in hand is shown, but money never moves into or out of it by hand.
  const active = data?.items.filter((l) => l.active && l.kind !== 'cheque') ?? []
  const canSettle = active.some((l) => l.kind === 'card') && active.some((l) => l.kind === 'bank')

  return (
    <>
      <PageHead title="Lockers" sub="Money kept outside the tills — a safe, a bank account, the owner's pocket."
        actions={<>
          {active.length >= 2 && <Button variant="outline" className="h-10 gap-1.5 px-4" onClick={() => setMove({ kind: 'transfer' })}><ArrowLeftRight size={16} /> Move money</Button>}
          {canSettle && <Button variant="outline" className="h-10 gap-1.5 px-4" onClick={() => setMove({ kind: 'settle' })}><CreditCard size={16} /> Settle card</Button>}
          <Button className="h-10 gap-1.5 px-4 font-bold" onClick={() => setEditing('new')}><Plus size={16} /> New locker</Button>
        </>} />
      <div className="relative mb-4 overflow-hidden rounded-[var(--radius)] bg-hero bg-[image:var(--hero-art)] p-5 text-hero-foreground md:p-6">
        <small className="text-[13px] font-semibold text-hero-muted">Across {active.length} active locker{active.length === 1 ? '' : 's'}</small>
        <div className="font-display text-3xl font-extrabold tabular-nums md:text-4xl">{data ? money(sym, data.total) : '…'}</div>
      </div>
      <div className="mb-3 flex justify-end">
        <button type="button" aria-pressed={disabled} onClick={() => void navigate({ search: { disabled: !disabled } })}
          className="rounded-pill border border-border bg-card px-3.5 py-2 text-[13px] font-semibold text-muted-foreground aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground">
          Show archived
        </button>
      </div>
      {data && data.items.length === 0 ? (
        <div className={SURFACE}><EmptyState title="No lockers yet">Add your safe, bank account or the owner's pocket to track money that leaves the tills.</EmptyState></div>
      ) : (
        <ul aria-label="Lockers" className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {data?.items.map((l) => <LockerTile key={l.id} l={l} sym={sym} onEdit={() => setEditing(l)} onArchive={() => setArchiving(l)}
            onMove={active.length >= 2 && l.kind !== 'card' && l.kind !== 'cheque' ? () => setMove({ kind: 'transfer', l }) : undefined} onAdjust={() => setMove({ kind: 'adjust', l })}
            onReactivate={() => void setActive(l, true).catch((e) => toast.error(errMsg(e)))} />)}
        </ul>
      )}
      {data && <TillCash sym={sym} lockers={active.filter(holdsCash)} value={data.tillCash} />}
      {move?.kind === 'transfer' && <TransferSheet lockers={active} from={move.l?.id} sym={sym} onClose={() => setMove(null)} />}
      {move?.kind === 'settle' && <SettleSheet lockers={active} sym={sym} onClose={() => setMove(null)} />}
      {move?.kind === 'adjust' && move.l && <AdjustSheet l={move.l} sym={sym} onClose={() => setMove(null)} />}
      {editing && <LockerForm l={editing === 'new' ? undefined : editing} onClose={() => setEditing(null)} />}
      <Dialog open={!!archiving} onOpenChange={(o) => !o && setArchiving(null)}>
        <DialogContent className="gap-3 p-5 sm:max-w-sm">
          <DialogTitle className="font-display text-lg font-bold">Archive {archiving?.name}?</DialogTitle>
          <DialogDescription>It leaves the pickers and the total. Its balance{archiving && num(archiving.balance) !== 0 ? ` of ${money(sym, archiving.balance)}` : ''} and history are kept, and you can bring it back.</DialogDescription>
          <div className="mt-2 flex justify-end gap-2">
            <Button variant="ghost" className="h-10 px-4" onClick={() => setArchiving(null)}>Keep it</Button>
            <Button className="h-10 px-4 font-bold" onClick={() => archiving && void setActive(archiving, false).then(() => setArchiving(null), (e) => toast.error(errMsg(e)))}>Archive</Button>
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}

function LockerTile({ l, sym, onEdit, onArchive, onReactivate, onMove, onAdjust }: {
  l: AdminLocker; sym: string; onEdit: () => void; onArchive: () => void; onReactivate: () => void; onMove?: () => void; onAdjust: () => void
}) {
  const { label, Icon } = kindOf(l.kind)
  return (
    <li className={cn(SURFACE, 'relative flex min-w-0 flex-col gap-3 p-4', !l.active && 'opacity-60')}>
      <div className="flex items-start gap-3">
        <span className="grid size-10 flex-none place-items-center rounded-full bg-brand-soft text-foreground"><Icon size={18} /></span>
        <div className="min-w-0 flex-1">
          <Link to="/admin/lockers/$id" params={{ id: String(l.id) }} className="block truncate font-bold outline-none after:absolute after:inset-0 focus-visible:underline">{l.name}</Link>
          <small className="text-[12px] text-muted-foreground">{label}</small>
        </div>
        {l.kind !== 'cheque' && <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" aria-label={`${l.name} actions`} className="relative z-10 size-9"><MoreHorizontal size={16} /></Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            {l.active && onMove && <DropdownMenuItem onSelect={onMove}><ArrowLeftRight size={15} /> Move money from here</DropdownMenuItem>}
            {l.active && <DropdownMenuItem onSelect={onAdjust}><SlidersHorizontal size={15} /> Adjust</DropdownMenuItem>}
            <DropdownMenuItem onSelect={onEdit}><Pencil size={15} /> Edit</DropdownMenuItem>
            {l.active
              ? <DropdownMenuItem onSelect={onArchive}><Archive size={15} /> Archive</DropdownMenuItem>
              : <DropdownMenuItem onSelect={onReactivate}><RotateCcw size={15} /> Bring back</DropdownMenuItem>}
          </DropdownMenuContent>
        </DropdownMenu>}
      </div>
      <Balance sym={sym} v={l.balance} className={cn('font-display text-2xl', l.kind === 'cheque' && 'text-cheque')} />
      {l.afterCheques && l.afterCheques !== l.balance && (
        <p className={cn('-mt-2 text-[13px] tabular-nums', num(l.afterCheques) < 0 ? 'font-semibold text-warning' : 'text-muted-foreground')}>
          After cheques {money(sym, l.afterCheques)}{num(l.afterCheques) < 0 && ' — below zero once your cheques are cashed'}
        </p>
      )}
      {l.kind === 'cheque' && <Link to="/admin/cheques" className="relative z-10 -mt-1 text-[13px] font-semibold text-cheque">Review cheques →</Link>}
      {(l.tillDefault || l.overdraft || !l.cashierAccess || !l.active) && (
        <div className="flex flex-wrap gap-1.5">
          {l.tillDefault && <Tag tone="brand">Till default</Tag>}
          {l.overdraft && <Tag tone="warn">Can go below zero</Tag>}
          {!l.cashierAccess && <Tag tone="muted">Owner only</Tag>}
          {!l.active && <Tag tone="muted">archived</Tag>}
        </div>
      )}
    </li>
  )
}

/** How the till's open / deposit / withdraw / close dialogs use lockers
 * (karots-pos Settings → "Cashier cash movements"). Saves as it changes. */
function TillCash({ sym, lockers, value }: { sym: string; lockers: AdminLocker[]; value: TillCashRequest }) {
  const qc = useQueryClient()
  const [busy, setBusy] = useState(false)
  const save = async (patch: Partial<TillCashRequest>) => {
    setBusy(true)
    try {
      await api('/admin/settings/till-cash', { method: 'PUT', json: { ...value, ...patch } })
      toast.success('Till cash settings saved')
      void qc.invalidateQueries({ queryKey: ['admin'] })
      void qc.invalidateQueries({ queryKey: ['till'] })
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <section aria-label="At the till" className={cn(SURFACE, 'mt-5 grid gap-4 p-5 md:grid-cols-2')}>
      <div className="md:col-span-2">
        <h2 className="font-display text-lg font-bold">At the till</h2>
        <p className="text-sm text-muted-foreground">When a cashier opens, closes, deposits or withdraws, which locker the cash goes to or comes from.</p>
      </div>
      <Field label="Locker picked by default" hint={lockers.length ? 'The cashier can still pick another.' : 'Add a locker first.'}>
        <select aria-label="Locker picked by default" disabled={busy || !lockers.length} value={value.defaultLockerId}
          onChange={(e) => void save({ defaultLockerId: Number(e.target.value) })} className={cn(INPUT, 'appearance-auto')}>
          <option value={0}>None — the cashier picks</option>
          {lockers.map((l) => <option key={l.id} value={l.id}>{l.name} ({money(sym, l.balance)})</option>)}
        </select>
      </Field>
      {/* Same label / control / hint shape as the select beside it, so the two line up. */}
      <div className="flex flex-col gap-1.5 text-[13px] font-semibold text-muted-foreground">
        Untracked cash
        <Toggle label="Cash may leave untracked" className="h-[var(--control-h)] py-0"
          checked={value.allowUntracked} onChange={(allowUntracked) => !busy && void save({ allowUntracked })} />
        <small className="font-normal text-faint">Off = a withdraw or close must name the locker it goes into. Opening and deposits are never blocked.</small>
      </div>
    </section>
  )
}
