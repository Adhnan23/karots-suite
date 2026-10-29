import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { ProductSaved, ServiceRequest } from '@/api/types.gen'
import { DataList } from '@/components/admin/DataList'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { EmptyState, PageHead } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { CategoryPicker } from '@/features/admin/categories/CategoryPicker'
import { ProductForm } from '@/features/admin/products/ProductForm'
import { Tag } from '@/features/admin/products/ProductsPage'
import { unitsQuery } from '@/features/admin/units/UnitsPage'
import { configQuery, errMsg } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { servicesQuery } from './queries'
import { RecipeEditor } from './RecipeEditor'

/** Non-stocked lines and what each one consumes. */
export function ServicesPage() {
  const rows = useQuery(servicesQuery).data
  const sym = useSuspenseQuery(configQuery).data.symbol
  const [creating, setCreating] = useState(false)
  const [recipe, setRecipe] = useState(0)
  const [edit, setEdit] = useState(0)
  return (
    <>
      <PageHead title="Services & recipes" sub="Things you sell without holding stock — a cup of tea, a photocopy — and the stock each one uses."
        actions={<Button className="h-10 gap-1.5 px-4 font-bold" onClick={() => setCreating(true)}><Plus size={16} /> New service</Button>} />
      <DataList label="Services" rows={rows ?? []} rowKey={(s) => s.id} onRow={(s) => setRecipe(s.id)}
        empty={<EmptyState title={rows ? 'No services yet' : 'Loading…'}>{rows && 'Create one to sell something that uses stock without holding any.'}</EmptyState>}
        columns={[
          { key: 'name', header: 'Service', cell: (s) => (
            <span className="flex flex-col"><span className="flex items-center gap-1.5"><b className={cn(!s.isActive && 'text-faint line-through')}>{s.name}</b>{!s.isActive && <Tag tone="muted">disabled</Tag>}</span>
              <small className="text-faint">{s.categoryName} · per {s.unitAbbr}</small></span>
          ) },
          { key: 'price', header: 'Price', num: true, cell: (s) => <b>{money(sym, s.sellingPrice)}</b> },
          { key: 'cost', header: 'Costs', num: true, cell: (s) => s.trueCost ? (
            <span className="flex flex-col items-end">{money(sym, s.trueCost)}{num(s.otherCost) > 0 && <small className="text-faint">{money(sym, s.stockCost)} stock + {money(sym, s.otherCost)} other</small>}</span>
          ) : <span className="text-faint">—</span> },
          { key: 'margin', header: 'Left over', num: true, cell: (s) => s.margin ? <b className={num(s.margin) < 0 ? 'text-danger' : 'text-success'}>{money(sym, s.margin)}</b> : <span className="text-faint">—</span> },
          { key: 'recipe', header: 'Recipe', cell: (s) => s.ingredients ? <Tag tone="brand">{`${s.ingredients} ingredient${s.ingredients === 1 ? '' : 's'}`}</Tag> : <span className="text-faint">none yet</span> },
          { key: 'edit', header: '', wide: true, interactive: true, className: 'w-[1%]', cell: (s) => <Button variant="ghost" className="h-9 px-3" onClick={() => setEdit(s.id)}>Edit</Button> },
        ]} />
      {creating && <NewService onClose={() => setCreating(false)} onCreated={(id) => { setCreating(false); setRecipe(id) }} />}
      {recipe > 0 && <RecipeEditor key={recipe} id={recipe} onClose={() => setRecipe(0)} />}
      {edit > 0 && <ProductForm key={edit} editing={{ id: edit }} onEdit={(e) => setEdit(e.id)} onClose={() => setEdit(0)} />}
    </>
  )
}

function NewService({ onClose, onCreated }: { onClose: () => void; onCreated: (id: number) => void }) {
  const units = useQuery(unitsQuery).data ?? []
  const qc = useQueryClient()
  const [f, setF] = useState<ServiceRequest>({ name: '', categoryId: 0, unitId: 0, sellingPrice: '' })
  const [busy, setBusy] = useState(false)
  const unitId = f.unitId || units.find((u) => u.abbreviation === 'pcs')?.id || units[0]?.id || 0
  async function save() {
    setBusy(true)
    try {
      const r = await api<ProductSaved>('/admin/services', { json: { ...f, unitId } })
      toast.success(`${r.message} — now say what it uses`)
      void qc.invalidateQueries({ queryKey: ['admin'] })
      onCreated(r.id)
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title="New service" submit="Add service" busy={busy} onSubmit={save}>
      <Field label="Name"><input autoFocus required maxLength={150} name="svc-name" placeholder="e.g. Tea, Photocopy A4" value={f.name} onChange={(e) => setF({ ...f, name: e.target.value })} className={INPUT} /></Field>
      <Field label="Category"><CategoryPicker label="Category" allowCreate value={f.categoryId} onChange={(categoryId) => setF({ ...f, categoryId })} /></Field>
      <div className="grid grid-cols-2 gap-3">
        <Field label="Sold per">
          <select aria-label="Unit" value={unitId} onChange={(e) => setF({ ...f, unitId: Number(e.target.value) })} className={cn(INPUT, 'appearance-auto')}>
            {units.map((u) => <option key={u.id} value={u.id}>{u.name}</option>)}
          </select>
        </Field>
        <Field label="Price"><input inputMode="decimal" aria-label="Price" placeholder="0.00" value={f.sellingPrice} onChange={(e) => setF({ ...f, sellingPrice: e.target.value.replace(/[^\d.]/g, '') })} className={cn(INPUT, 'tabular-nums')} /></Field>
      </div>
    </FormSheet>
  )
}
