import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { ArrowDown, ArrowLeft, ArrowUp, Plus, X } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { ApiError } from '@/api/client'
import type { AdminProductRow } from '@/api/types.gen'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import { ProductPicker } from '@/features/admin/products/ProductPicker'
import { Tag } from '@/features/admin/products/ProductsPage'
import { errMsg } from '@/features/till/api'
import { qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import {
  addMember, createTier, deleteGroup, deleteTier, groupQuery, membershipQuery, moveTier, removeMember, renameGroup,
  setGroupActive, setTierActive, updateTier,
} from './api'
import { hasTiers } from './labels'
import type { Group, Tier } from './types'

const SMALL = cn(INPUT, 'h-9 text-[14px]')
const ICON = 'grid size-8 place-items-center rounded-full text-muted-foreground hover:bg-muted disabled:opacity-30'

async function act(p: Promise<unknown>, done?: string) {
  try {
    await p
    if (done) toast.success(done)
    return true
  } catch (e) {
    toast.error(errMsg(e))
    return false
  }
}

const num = (s: string) => Math.max(0, Math.floor(Number(s) || 0))

/** A tier's name and reorder level, saved together (Save shows once changed). A
 * group without named tiers edits only the level. */
function TierHead({ t, tiered, first, last, canDelete }: { t: Tier; tiered: boolean; first: boolean; last: boolean; canDelete: boolean }) {
  const [name, setName] = useState(t.name)
  const [level, setLevel] = useState(String(t.reorderLevel))
  const dirty = name.trim() !== t.name || num(level) !== t.reorderLevel
  const save = () => void act(updateTier(t.id, name.trim(), num(level)), 'Saved')
  const levelInput = (
    <input aria-label={tiered ? `Reorder level of ${t.name}` : 'Reorder level'} inputMode="numeric" value={level}
      onChange={(e) => setLevel(e.target.value.replace(/\D/g, ''))} className={cn(SMALL, 'w-16 text-right tabular-nums')} />
  )
  return (
    <form className="flex flex-wrap items-center gap-x-1.5 gap-y-2 md:gap-2" onSubmit={(e) => { e.preventDefault(); if (dirty) save() }}>
      {tiered ? (
        <>
          <input aria-label="Tier name" value={name} onChange={(e) => setName(e.target.value)} placeholder="Name this tier"
            className={cn(SMALL, 'min-w-0 basis-full font-display text-[15px] font-bold md:max-w-sm md:flex-1 md:basis-auto')} />
          <label className="flex items-center gap-1.5 text-[13px] text-muted-foreground">Reorder at ≤ {levelInput}</label>
        </>
      ) : (
        <label className="flex flex-1 flex-wrap items-center gap-2 text-[13.5px] text-muted-foreground">Reorder when the group has ≤ {levelInput}
          <small className="basis-full text-faint">0 = don't track. Reorder then lists this group's products when their total runs down.</small>
        </label>
      )}
      {dirty && <Button type="submit" className="h-9 px-3 font-bold">Save</Button>}
      {tiered && (
        <span className="ml-auto flex items-center">
          <button type="button" aria-label={`Move ${t.name} up`} disabled={first} className={ICON} onClick={() => void act(moveTier(t.id, true))}><ArrowUp size={16} /></button>
          <button type="button" aria-label={`Move ${t.name} down`} disabled={last} className={ICON} onClick={() => void act(moveTier(t.id, false))}><ArrowDown size={16} /></button>
          <Button type="button" variant="ghost" className="h-9 px-2" onClick={() => void act(setTierActive(t.id, !t.active), t.active ? 'Tier turned off' : 'Tier turned on')}>
            {t.active ? 'Turn off' : 'Turn on'}
          </Button>
          {canDelete && <ConfirmButton label={<X size={16} />} title={`Delete ${t.name || 'this tier'}?`} confirm="Delete" className="size-9 p-0"
            onConfirm={() => act(deleteTier(t.id), 'Tier deleted')} />}
        </span>
      )}
    </form>
  )
}

function TierCard({ g, t, i, tiered, onPick }: { g: Group; t: Tier; i: number; tiered: boolean; onPick: (t: Tier, p: AdminProductRow) => void }) {
  // A member sold by another unit than the group's first (a loose version) shows it:
  // the till asks how many when swapping to it.
  const baseUnit = g.tiers.flatMap((x) => x.members)[0]?.unit ?? ''
  const where = useQuery(membershipQuery).data ?? {}
  const mine = new Set(g.tiers.flatMap((x) => x.members.map((m) => m.productId)))
  return (
    <section aria-label={t.name || 'Products'} className={cn(SURFACE, 'flex flex-col gap-3 p-4', !t.active && 'opacity-70')}>
      <TierHead key={`${t.name}|${t.reorderLevel}`} t={t} tiered={tiered} first={i === 0} last={i === g.tiers.length - 1}
        canDelete={g.tiers.length > 1 && t.members.length === 0} />
      <div className="flex flex-wrap items-center gap-2 text-[12.5px] text-muted-foreground tabular-nums">
        <span><b className="text-foreground">{qtyText(t.total)}</b> in stock</span>
        {t.low && <Tag tone="danger">Low</Tag>}
        {!t.active && <Tag tone="muted">Off — the till doesn't offer these</Tag>}
      </div>
      <ul className="flex flex-col divide-y divide-border">
        {t.members.map((m) => (
          <li key={m.productId} className={cn('flex items-center gap-3 py-2 text-[14px]', !m.active && 'text-faint')}>
            <span className="flex min-w-0 flex-1 flex-col items-start gap-0.5">
              <span className="flex max-w-full items-center gap-1.5">
                <span className="truncate font-semibold">{m.name}</span>
                {m.unit && m.unit !== baseUnit && <Tag tone="muted">{m.unit}</Tag>}
              </span>
              {!m.active && <Tag tone="warn">Disabled — not counted</Tag>}
            </span>
            <span className="shrink-0 tabular-nums text-muted-foreground">{qtyText(m.stock)} left</span>
            <button type="button" aria-label={`Remove ${m.name}`} className="text-faint hover:text-danger" onClick={() => void act(removeMember(m.productId), `${m.name} removed`)}><X size={16} /></button>
          </li>
        ))}
        {t.members.length === 0 && <li className="py-2 text-[13.5px] text-faint">No products here yet.</li>}
      </ul>
      <ProductPicker title={`Add to ${t.name || g.name}`} exclude={mine} onPick={(p) => onPick(t, p)}
        hint={(p) => { const w = where[String(p.id)]; return w ? `In ${w.group}${w.tier ? ` · ${w.tier}` : ''}` : undefined }}
        trigger={(o) => <Button type="button" variant="secondary" className="h-10 self-start" onClick={o}><Plus size={15} /> Add products</Button>} />
    </section>
  )
}

/** Add a tier. A group without named tiers names its first tier in the same step. */
function AddTier({ g, onClose }: { g: Group; onClose: () => void }) {
  const blank = g.tiers.length === 1 && g.tiers[0].name === '' ? g.tiers[0] : null
  const [first, setFirst] = useState('')
  const [name, setName] = useState('')
  const [level, setLevel] = useState('0')
  const [busy, setBusy] = useState(false)
  const submit = async () => {
    setBusy(true)
    const ok = (!blank || await act(updateTier(blank.id, first.trim(), blank.reorderLevel))) && await act(createTier(g.id, name.trim(), num(level)), 'Tier added')
    setBusy(false)
    if (ok) onClose()
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title="Add a quality tier" submit="Add tier" busy={busy} onSubmit={() => void submit()}>
      {blank && (
        <Field label="The products already here are" hint="Best first: the till offers the top tier first.">
          <input required autoFocus value={first} onChange={(e) => setFirst(e.target.value)} placeholder="Genuine" className={INPUT} />
        </Field>
      )}
      <Field label="New tier" hint="It goes below the others; move it with ↑ ↓.">
        <input required autoFocus={!blank} value={name} onChange={(e) => setName(e.target.value)} placeholder="Cheap" className={INPUT} />
      </Field>
      <Field label="Reorder when this tier has ≤" hint="0 = don't track.">
        <input inputMode="numeric" value={level} onChange={(e) => setLevel(e.target.value.replace(/\D/g, ''))} className={INPUT} />
      </Field>
    </FormSheet>
  )
}

function Rename({ g, onClose }: { g: Group; onClose: () => void }) {
  const [name, setName] = useState(g.name)
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title="Rename group" submit="Save" onSubmit={() => void act(renameGroup(g.id, name), 'Saved').then((ok) => ok && onClose())}>
      <Field label="Name"><input required autoFocus value={name} onChange={(e) => setName(e.target.value)} className={INPUT} /></Field>
    </FormSheet>
  )
}

/** One group: its tiers (best first), their products and reorder levels. */
export function GroupPage({ id }: { id: number }) {
  const q = useQuery(groupQuery(id))
  const g = q.data
  const navigate = useNavigate()
  const [sheet, setSheet] = useState<'tier' | 'rename' | null>(null)
  const [move, setMove] = useState<{ t: Tier; p: AdminProductRow; msg: string } | null>(null)
  if (q.isError) return <EmptyState title="That group is gone"><Link to="/admin/x/$key/$" params={{ key: 'alternatives', _splat: '' }} className="text-brand">← All groups</Link></EmptyState>
  if (!g) return null
  const tiered = hasTiers(g)
  const pick = async (t: Tier, p: AdminProductRow) => {
    try {
      await addMember(t.id, p.id)
      toast.success(`${p.name} added`)
    } catch (e) {
      if (e instanceof ApiError && e.code === 'IN_OTHER_GROUP') setMove({ t, p, msg: e.message })
      else toast.error(errMsg(e))
    }
  }
  return (
    <>
      <PageHead title={g.name}
        sub={<Link to="/admin/x/$key/$" params={{ key: 'alternatives', _splat: '' }} className="inline-flex items-center gap-1 text-brand"><ArrowLeft size={14} /> All groups · {g.products} product{g.products === 1 ? '' : 's'}</Link>}
        actions={<>
          <Button variant="secondary" className="h-10 px-4" onClick={() => setSheet('rename')}>Rename</Button>
          <Button variant="ghost" className="h-10 px-4" onClick={() => void act(setGroupActive(g.id, !g.active), g.active ? 'Group turned off' : 'Group turned on')}>{g.active ? 'Turn off' : 'Turn on'}</Button>
          <ConfirmButton label="Delete" title={`Delete ${g.name}?`} confirm="Delete" body="Its products stay in the catalogue; they just stop being alternatives."
            onConfirm={async () => { await deleteGroup(g.id); toast.success('Group deleted'); void navigate({ to: '/admin/x/$key/$', params: { key: 'alternatives', _splat: '' } }) }} />
        </>} />
      {!g.active && <p className="mb-3 rounded-[var(--radius-sm)] bg-muted px-3 py-2 text-[13.5px] text-muted-foreground">This group is off: the till doesn't search, pin or swap it, and Reorder ignores it.</p>}
      <div className="flex flex-col gap-3">
        {g.tiers.map((t, i) => <TierCard key={t.id} g={g} t={t} i={i} tiered={tiered} onPick={(t, p) => void pick(t, p)} />)}
        <Button variant="ghost" className="h-10 self-start px-3 text-brand" onClick={() => setSheet('tier')}><Plus size={15} /> Add a quality tier</Button>
      </div>
      {sheet === 'tier' && <AddTier g={g} onClose={() => setSheet(null)} />}
      {sheet === 'rename' && <Rename g={g} onClose={() => setSheet(null)} />}
      <Dialog open={!!move} onOpenChange={(o) => !o && setMove(null)}>
        <DialogContent className="gap-3 p-5 sm:max-w-sm">
          <DialogTitle className="font-display text-lg font-bold">Move {move?.p.name} here?</DialogTitle>
          <DialogDescription>{move?.msg}. A product sits in one group only.</DialogDescription>
          <div className="mt-2 flex justify-end gap-2">
            <Button variant="ghost" className="h-10 px-4" onClick={() => setMove(null)}>Leave it</Button>
            <Button className="h-10 px-4 font-bold" onClick={() => { if (move) void act(addMember(move.t.id, move.p.id, true), `${move.p.name} moved here`); setMove(null) }}>Move here</Button>
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}
