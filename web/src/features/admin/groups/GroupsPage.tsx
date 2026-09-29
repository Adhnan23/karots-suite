import { queryOptions, useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { ArrowDown, ArrowUp, Plus, X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { GroupItem, GroupNode, GroupRequest } from '@/api/types.gen'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { EmojiPicker } from '@/components/admin/EmojiPicker'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { ProductPicker } from '@/features/admin/products/ProductPicker'
import { Tag } from '@/features/admin/products/ProductsPage'
import { configQuery, errMsg } from '@/features/till/api'
import { money, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'

const groupsQuery = (disabled: boolean) => queryOptions({ queryKey: ['admin', 'groups', disabled], queryFn: () => api<GroupNode[]>(`/admin/groups?disabled=${disabled ? 1 : 0}`) })
const itemsQuery = (id: number) => queryOptions({ queryKey: ['admin', 'groups', 'items', id], queryFn: () => api<GroupItem[]>(`/admin/groups/${id}/items`) })

type Editing = { node: GroupNode | null; parentId: number }

/** The till's menu cards (karots-pos "Cashier Menu"): nested groups, their
 * order, and the products on each card with an emoji. */
export function GroupsPage() {
  const [disabled, setDisabled] = useState(false)
  const groups = useQuery(groupsQuery(disabled)).data
  const [picked, setPicked] = useState(0)
  const [editing, setEditing] = useState<Editing | null>(null)
  const qc = useQueryClient()
  const cur = groups?.find((g) => g.id === picked)
  const refresh = () => qc.invalidateQueries({ queryKey: ['admin', 'groups'] })
  const run = async (fn: () => Promise<unknown>) => { try { await fn(); await refresh() } catch (e) { toast.error(errMsg(e)) } }
  const siblings = (g: GroupNode) => (groups ?? []).filter((x) => x.parentId === g.parentId)

  return (
    <>
      <PageHead title="Cashier menu" sub="The group cards on the till. Tap a group to choose which products show on it."
        actions={<>
          <button type="button" aria-pressed={disabled} onClick={() => setDisabled(!disabled)}
            className="rounded-full border border-border bg-card px-3.5 py-2 text-[13px] font-semibold text-muted-foreground aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground">Show disabled</button>
          <Button className="h-10 gap-1.5 px-4 font-bold" onClick={() => setEditing({ node: null, parentId: 0 })}><Plus size={16} /> New group</Button>
        </>} />
      <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_minmax(0,1.25fr)] md:items-start">
        <div className={cn(SURFACE, 'overflow-hidden')}>
          {!groups?.length ? <EmptyState title={groups ? 'No groups yet' : 'Loading…'}>{groups && 'Make a card for each shelf — Drinks, Snacks — then add the products the cashier reaches for.'}</EmptyState> : (
            <ul aria-label="Groups" className="divide-y divide-border">
              {groups.map((g) => {
                const sib = siblings(g)
                const at = sib.indexOf(g)
                return (
                  <li key={g.id} className={cn('group flex items-center gap-1 pr-2', picked === g.id ? 'bg-selected text-selected-foreground' : 'hover:bg-muted')} style={{ paddingLeft: 10 + g.depth * 22 }}>
                    <button type="button" aria-pressed={picked === g.id} onClick={() => setPicked(g.id)} className="flex min-w-0 flex-1 items-center gap-2.5 py-2.5 text-left">
                      <span className="grid h-9 w-9 flex-none place-items-center rounded-[10px] bg-muted text-lg">{g.emoji || '📦'}</span>
                      <span className="min-w-0">
                        <b className={cn('block truncate', !g.isActive && 'line-through opacity-60')}>{g.name}</b>
                        <small className="opacity-70">{g.itemCount} {g.itemCount === 1 ? 'product' : 'products'}{g.childCount > 0 && ` · ${g.childCount} inside`}</small>
                      </span>
                      {!g.isActive && <Tag tone="muted">disabled</Tag>}
                    </button>
                    <span className={cn('flex flex-none', picked !== g.id && 'md:opacity-0 md:group-hover:opacity-100 md:focus-within:opacity-100')}>
                      <IconBtn label={`Move ${g.name} up`} disabled={at <= 0} onClick={() => run(() => api(`/admin/groups/${g.id}/move?dir=up`, { method: 'POST' }))}><ArrowUp size={15} /></IconBtn>
                      <IconBtn label={`Move ${g.name} down`} disabled={at >= sib.length - 1} onClick={() => run(() => api(`/admin/groups/${g.id}/move?dir=down`, { method: 'POST' }))}><ArrowDown size={15} /></IconBtn>
                      <IconBtn label={`Add a group inside ${g.name}`} onClick={() => setEditing({ node: null, parentId: g.id })}><Plus size={15} /></IconBtn>
                    </span>
                  </li>
                )
              })}
            </ul>
          )}
        </div>
        {cur ? <Items key={cur.id} g={cur} onEdit={() => setEditing({ node: cur, parentId: cur.parentId })} onChanged={refresh}
          onEnable={() => run(() => api(`/admin/groups/${cur.id}/activate`, { method: 'POST' }))} /> : (
          <div className={cn(SURFACE, 'hidden md:block')}><EmptyState title="Pick a group">Its products — the buttons the cashier taps — show here.</EmptyState></div>
        )}
      </div>
      {editing && <GroupForm key={editing.node?.id ?? `new-${editing.parentId}`} {...editing} groups={groups ?? []}
        onClose={() => setEditing(null)} onDone={(id) => { setEditing(null); void refresh(); if (id) setPicked(id) }} />}
    </>
  )
}

function IconBtn({ label, disabled, onClick, children }: { label: string; disabled?: boolean; onClick: () => void; children: React.ReactNode }) {
  return <button type="button" aria-label={label} title={label} disabled={disabled} onClick={onClick} className="grid h-8 w-8 place-items-center rounded-full enabled:hover:bg-card disabled:opacity-30">{children}</button>
}

function Items({ g, onEdit, onEnable, onChanged }: { g: GroupNode; onEdit: () => void; onEnable: () => void; onChanged: () => void }) {
  const items = useQuery(itemsQuery(g.id)).data
  const sym = useSuspenseQuery(configQuery).data.symbol
  const qc = useQueryClient()
  const reload = () => { void qc.invalidateQueries({ queryKey: ['admin', 'groups', 'items', g.id] }); onChanged() }
  const ref = useRef<HTMLElement>(null)
  // On a phone the pane sits under the whole tree: bring it into view when a group is picked (keyed by group, so once).
  useEffect(() => { if (window.matchMedia('(max-width: 767px)').matches) ref.current?.scrollIntoView({ behavior: 'smooth', block: 'start' }) }, [])
  const act = async (fn: () => Promise<unknown>) => { try { await fn(); reload() } catch (e) { toast.error(errMsg(e)) } }
  return (
    <section ref={ref} aria-label={`${g.name} products`} className={cn(SURFACE, 'flex scroll-mt-4 flex-col overflow-hidden')}>
      <header className="flex flex-wrap items-center gap-3 border-b border-border p-4">
        <span className="grid h-12 w-12 place-items-center rounded-[14px] bg-brand-soft text-2xl">{g.emoji || '📦'}</span>
        <span className="min-w-0 flex-1">
          <b className="block truncate font-display text-lg">{g.name}</b>
          <small className="text-muted-foreground">{items ? `${items.length} ${items.length === 1 ? 'product' : 'products'} on this card` : '…'}</small>
        </span>
        {!g.isActive && <Button variant="outline" className="h-9 px-3" onClick={onEnable}>Enable</Button>}
        <Button variant="ghost" className="h-9 px-3" onClick={onEdit}>Edit</Button>
      </header>
      <ul className="flex flex-col divide-y divide-border">
        {items?.map((p) => (
          <li key={p.productId} className="flex items-center gap-2.5 px-4 py-2">
            <EmojiPicker label={`Emoji for ${p.name}`} value={p.emoji} className="h-10 w-10 flex-none"
              onChange={(emoji) => act(() => api(`/admin/groups/${g.id}/items/${p.productId}/emoji`, { method: 'PUT', json: { productId: p.productId, emoji } }))} />
            <span className="min-w-0 flex-1">
              <b className="block truncate text-sm">{p.name}</b>
              <small className="text-faint tabular-nums">{money(sym, p.sellingPrice)}{!p.isService && ` · ${qtyText(p.stockQty)} ${p.unitAbbr} on hand`}</small>
            </span>
            <button type="button" aria-label={`Take ${p.name} off this card`} onClick={() => act(() => api(`/admin/groups/${g.id}/items/${p.productId}`, { method: 'DELETE' }))}
              className="grid h-8 w-8 place-items-center rounded-full text-faint hover:bg-muted hover:text-danger"><X size={16} /></button>
          </li>
        ))}
        {items?.length === 0 && <li className="p-6 text-center text-sm text-faint">No products on this card yet.</li>}
      </ul>
      <div className="p-3">
        <ProductPicker title={`Add to ${g.name}`} exclude={new Set(items?.map((p) => p.productId))}
          onPick={(p) => act(() => api(`/admin/groups/${g.id}/items`, { json: { productId: p.id, emoji: '' } }))}
          trigger={(open) => <Button variant="outline" className="h-10 w-full gap-1.5" onClick={open}><Plus size={15} /> Add a product</Button>} />
      </div>
    </section>
  )
}

function GroupForm({ node, parentId, groups, onClose, onDone }: Editing & { groups: GroupNode[]; onClose: () => void; onDone: (id?: number) => void }) {
  const [f, setF] = useState<GroupRequest>({ name: node?.name ?? '', emoji: node?.emoji ?? '', parentId })
  const [busy, setBusy] = useState(false)
  const parent = groups.find((g) => g.id === parentId)
  async function save() {
    setBusy(true)
    try {
      if (node) { await api(`/admin/groups/${node.id}`, { method: 'PUT', json: f }); onDone() }
      else onDone((await api<{ id: number }>('/admin/groups', { json: f })).id)
      toast.success(node ? 'Group saved' : 'Group added')
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} busy={busy} onSubmit={save}
      title={node ? `Edit ${node.name}` : parent ? `New group in ${parent.name}` : 'New group'} submit={node ? 'Save' : 'Add group'}
      extra={node && (
        <ConfirmButton label="Delete" title={`Delete ${node.name}?`} confirm="Delete group"
          body={`${node.childCount ? `Its ${node.childCount} groups inside go too. ` : ''}The products stay in the catalog; they just leave this card.`}
          onConfirm={async () => { await api(`/admin/groups/${node.id}`, { method: 'DELETE' }); toast.success('Group deleted'); onDone() }} />
      )}>
      <div className="grid grid-cols-[76px_1fr] gap-3">
        <Field label="Emoji"><EmojiPicker label="Group emoji" value={f.emoji} onChange={(emoji) => setF({ ...f, emoji })} className="h-[var(--control-h)] w-full" /></Field>
        <Field label="Name"><input autoFocus required maxLength={60} name="grp-name" value={f.name} onChange={(e) => setF({ ...f, name: e.target.value })} placeholder="Cold drinks" className={INPUT} /></Field>
      </div>
      <div className="flex flex-wrap gap-1.5" aria-label="Quick emoji">
        {['🥤', '🍞', '🍚', '🥛', '🍫', '🧃', '🧼', '🧴', '✏️', '📱', '💊', '🍪'].map((e) => (
          <button key={e} type="button" aria-pressed={f.emoji === e} onClick={() => setF({ ...f, emoji: e })} className="grid h-9 w-9 place-items-center rounded-[10px] bg-muted text-lg aria-pressed:ring-2 aria-pressed:ring-selected">{e}</button>
        ))}
      </div>
    </FormSheet>
  )
}
