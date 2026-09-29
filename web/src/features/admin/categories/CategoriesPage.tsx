import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronRight, FolderTree, Plus, Search } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { CategoryNode, CategoryRequest } from '@/api/types.gen'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'
import { CategoryPicker } from './CategoryPicker'
import { categoriesQuery } from './queries'
import { subtree, visibleTree } from './tree'

type Editing = { node: CategoryNode | null; parentId: number }

/** The nested category tree (karots-pos /admin/categories): collapse, search,
 * add a sub-category in place, rename or move a branch, delete an empty one. */
export function CategoriesPage() {
  const nodes = useQuery(categoriesQuery).data
  const [q, setQ] = useState('')
  const [collapsed, setCollapsed] = useState<Set<number>>(new Set())
  const [editing, setEditing] = useState<Editing | null>(null)
  const rows = visibleTree(nodes ?? [], q, collapsed)
  const products = (nodes ?? []).reduce((s, n) => s + n.productCount, 0)
  const toggle = (id: number) => setCollapsed((c) => { const n = new Set(c); if (!n.delete(id)) n.add(id); return n })
  const withKids = (nodes ?? []).filter((n) => n.childCount > 0).map((n) => n.id)

  return (
    <>
      <PageHead title="Categories" sub={nodes && `${nodes.length} categories · ${products} products filed`}
        actions={<Button className="h-10 gap-1.5 px-4 font-bold" onClick={() => setEditing({ node: null, parentId: 0 })}><Plus size={16} /> New category</Button>} />
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <div className="relative min-w-[220px] flex-1 md:max-w-sm">
          <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
          <input aria-label="Search categories" placeholder="Search categories" value={q} onChange={(e) => setQ(e.target.value)} className={cn(INPUT, 'pl-9')} />
        </div>
        {!q && withKids.length > 0 && (
          <button type="button" onClick={() => setCollapsed(collapsed.size ? new Set() : new Set(withKids))}
            className="rounded-full border border-border bg-card px-3.5 py-2 text-[13px] font-semibold text-muted-foreground hover:text-foreground">
            {collapsed.size ? 'Expand all' : 'Collapse all'}
          </button>
        )}
      </div>
      <div className={cn(SURFACE, 'overflow-hidden')}>
        {rows.length === 0 ? (
          <EmptyState title={!nodes ? 'Loading…' : q ? `No category matches “${q}”` : 'No categories yet'}>
            {nodes && !q && 'Group products the way you shelve them — Groceries › Rice, Stationery › Pens.'}
          </EmptyState>
        ) : (
          <ul aria-label="Categories" className="divide-y divide-border">
            {rows.map((n) => (
              <li key={n.id} className="group flex items-center gap-1 pr-2 hover:bg-muted" style={{ paddingLeft: 8 + n.depth * 22 }}>
                <button type="button" aria-label={collapsed.has(n.id) ? `Expand ${n.name}` : `Collapse ${n.name}`} disabled={!n.childCount || !!q}
                  onClick={() => toggle(n.id)} className="grid h-8 w-8 flex-none place-items-center rounded-full text-faint enabled:hover:bg-card disabled:opacity-0">
                  <ChevronRight size={16} className={cn('transition-transform', !collapsed.has(n.id) && 'rotate-90')} />
                </button>
                <button type="button" onClick={() => setEditing({ node: n, parentId: n.parentId })} className="flex min-w-0 flex-1 items-center gap-2.5 py-2.5 text-left">
                  {n.depth === 0 && <FolderTree size={17} className="flex-none text-muted-foreground" />}
                  <span className={cn('truncate', n.depth === 0 ? 'font-bold' : 'font-medium')}>{n.name}</span>
                  {n.childCount > 0 && collapsed.has(n.id) && <small className="flex-none text-faint">+{n.childCount}</small>}
                  <span className={cn('ml-auto flex-none rounded-full px-2 py-0.5 text-[12px] font-semibold tabular-nums', n.productCount ? 'bg-brand-soft text-foreground' : 'text-faint')}>
                    {n.productCount} {n.productCount === 1 ? 'product' : 'products'}
                  </span>
                </button>
                <button type="button" aria-label={`Add under ${n.name}`} title="Add a sub-category" onClick={() => setEditing({ node: null, parentId: n.id })}
                  className="grid h-8 w-8 flex-none place-items-center rounded-full text-faint hover:bg-card hover:text-foreground md:opacity-0 md:group-hover:opacity-100 md:focus-visible:opacity-100">
                  <Plus size={16} />
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
      {editing && <CategoryForm key={editing.node?.id ?? `new-${editing.parentId}`} {...editing} nodes={nodes ?? []} onClose={() => setEditing(null)} />}
    </>
  )
}

function CategoryForm({ node, parentId, nodes, onClose }: Editing & { nodes: CategoryNode[]; onClose: () => void }) {
  const qc = useQueryClient()
  const [f, setF] = useState<CategoryRequest>({ name: node?.name ?? '', parentId })
  const [busy, setBusy] = useState(false)
  const done = (msg: string) => {
    toast.success(msg)
    void qc.invalidateQueries({ queryKey: ['admin'] })
    onClose()
  }
  async function save() {
    setBusy(true)
    try {
      if (node) await api(`/admin/categories/${node.id}`, { method: 'PUT', json: f })
      else await api('/admin/categories', { json: f })
      done(node ? 'Category saved' : 'Category added')
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  const parentName = nodes.find((n) => n.id === parentId)?.name
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} busy={busy} onSubmit={save}
      title={node ? `Edit ${node.name}` : parentName ? `New category in ${parentName}` : 'New category'} submit={node ? 'Save' : 'Add category'}
      extra={node && (
        <ConfirmButton label="Delete" title={`Delete ${node.name}?`} confirm="Delete category"
          body={node.productCount > 0 ? `${node.productCount} products are filed here — move them to another category first, or the delete is refused.`
            : node.childCount > 0 ? `Its ${node.childCount} sub-categories move up to the top level.` : 'Nothing is filed here.'}
          onConfirm={async () => { await api(`/admin/categories/${node.id}`, { method: 'DELETE' }); done('Category deleted') }} />
      )}>
      <Field label="Name"><input autoFocus required maxLength={80} name="cat-name" value={f.name} onChange={(e) => setF({ ...f, name: e.target.value })} className={INPUT} placeholder="Rice & grains" /></Field>
      <Field label="Sits under" hint="Leave on Top level for a main category.">
        <CategoryPicker label="Parent category" noneLabel="Top level" value={f.parentId} onChange={(id) => setF({ ...f, parentId: id })}
          exclude={node ? subtree(nodes, node.id) : undefined} />
      </Field>
    </FormSheet>
  )
}
