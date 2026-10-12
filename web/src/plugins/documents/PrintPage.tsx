import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { BarChart3, Plus } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { CategoryPicker } from '@/features/admin/categories/CategoryPicker'
import { ProductForm } from '@/features/admin/products/ProductForm'
import { Tag } from '@/features/admin/products/ProductsPage'
import { servicesQuery as coreServicesQuery } from '@/features/admin/services/queries'
import { RecipeEditor } from '@/features/admin/services/RecipeEditor'
import { configQuery, errMsg } from '@/features/till/api'
import { Seg } from '@/features/till/bits'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { createService, servicesQuery, setActive } from './api'
import { GridSheet } from './GridSheet'
import { CATEGORY, EMOJI, KIND } from './labels'
import { PrintReport } from './Report'
import type { Category, Kind, Service } from './types'

async function act(p: Promise<unknown>, done: string) {
  try {
    await p
    toast.success(done)
  } catch (e) {
    toast.error(errMsg(e))
  }
}

function problems(s: Service): string[] {
  if (s.kind === 'custom') return []
  const out = []
  if (!s.prices) out.push("No prices — the till can't sell it")
  if (!s.paper) out.push('No paper — sheets won’t come off stock')
  return out
}

function ServiceCard({ s, sym, open }: { s: Service; sym: string; open: (what: 'grid' | 'recipe' | 'edit') => void }) {
  const on = s.active && s.productActive
  return (
    <article className={cn(SURFACE, 'flex min-w-0 flex-col gap-3 p-4', !on && 'opacity-75')}>
      <header className="flex items-start gap-3">
        <span aria-hidden className="grid size-11 shrink-0 place-items-center rounded-[var(--radius-sm)] bg-brand-soft text-[22px]">{EMOJI[s.category]}</span>
        <div className="min-w-0 flex-1">
          <h2 className="break-words font-display text-[17px] font-bold leading-snug">{s.name}</h2>
          <div className="mt-1 flex flex-wrap gap-1.5">
            <Tag tone="brand">{s.kind === 'custom' ? 'Price at the till' : `${s.prices} price${s.prices === 1 ? '' : 's'}`}</Tag>
            {s.kind === 'metered' && s.paper > 0 && <Tag tone="muted">{`${s.paper} paper`}</Tag>}
            {!s.active && <Tag tone="muted">Off</Tag>}
            {s.active && !s.productActive && <Tag tone="muted">Service disabled</Tag>}
          </div>
        </div>
        {s.kind === 'custom' && Number(s.sellingPrice) > 0 && <b className="shrink-0 tabular-nums">{money(sym, s.sellingPrice)}</b>}
      </header>
      {problems(s).length > 0 && <div className="flex flex-wrap gap-1.5">{problems(s).map((p) => <Tag key={p} tone="warn">{p}</Tag>)}</div>}
      <footer className="mt-auto flex flex-wrap items-center gap-2 border-t border-border pt-3">
        <Button variant="secondary" className="h-9 px-3" onClick={() => open('grid')}>{s.kind === 'custom' ? 'Pricing' : 'Prices & paper'}</Button>
        {s.kind === 'metered' && <Button variant="ghost" className="h-9 px-3" onClick={() => open('recipe')}>Toner &amp; ink</Button>}
        <Button variant="ghost" className="h-9 px-3" onClick={() => open('edit')}>Edit</Button>
        <span className="flex-1" />
        <Button variant="ghost" className="h-9 px-3" onClick={() => void act(setActive(s.id, !s.active), s.active ? `${s.name} is off the till` : `${s.name} is on the till`)}>
          {s.active ? 'Turn off' : 'Turn on'}
        </Button>
      </footer>
    </article>
  )
}

function NewService({ onClose, onMade }: { onClose: () => void; onMade: (id: number) => void }) {
  const core = useQuery(coreServicesQuery).data ?? []
  const linked = new Set((useQuery(servicesQuery).data ?? []).map((s) => s.productId))
  const free = core.filter((s) => !linked.has(s.id))
  const [from, setFrom] = useState(0)
  const [name, setName] = useState('')
  const [categoryId, setCategoryId] = useState(0)
  const [kind, setKind] = useState<Kind>('metered')
  const [category, setCategory] = useState<Category>('copy')
  const [busy, setBusy] = useState(false)
  async function save() {
    setBusy(true)
    try {
      const r = await createService(from ? { productId: from, kind, category } : { name, categoryId, kind, category })
      toast.success(kind === 'metered' ? 'Print service added — now set its prices and paper' : 'Print service added')
      onMade(r.id)
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title="New print service" submit="Add" busy={busy} onSubmit={() => void save()}>
      {free.length > 0 && (
        <Field label="Start from" hint="A service you already have in Services & recipes, or a new one.">
          <select aria-label="Start from" value={from} onChange={(e) => setFrom(Number(e.target.value))} className={cn(INPUT, 'appearance-auto')}>
            <option value={0}>A new service</option>
            {free.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
          </select>
        </Field>
      )}
      {!from && (
        <>
          <Field label="Name"><input autoFocus required maxLength={150} name="doc-svc-name" placeholder="e.g. Photocopy, Print, Laminate, CV typing" value={name} onChange={(e) => setName(e.target.value)} className={INPUT} /></Field>
          <Field label="Category"><CategoryPicker label="Category" allowCreate value={categoryId} onChange={setCategoryId} /></Field>
        </>
      )}
      <Field label="Priced"><Seg label="Priced" value={kind} options={KIND} onChange={setKind} /></Field>
      <Field label="Kind of job"><Seg label="Kind of job" value={category} options={CATEGORY} onChange={setCategory} /></Field>
    </FormSheet>
  )
}

/** Apps → Print & Copy: the print services (core services with a price grid and
 * paper by size). The sub-path report is the copies report. */
export function PrintPage({ path }: { path: string }) {
  const sym = useSuspenseQuery(configQuery).data.symbol
  const list = useQuery(servicesQuery).data
  const [creating, setCreating] = useState(false)
  const [open, setOpen] = useState<{ id: number; what: 'grid' | 'recipe' | 'edit' } | null>(null)
  if (path === 'report') return <PrintReport />
  const cur = list?.find((s) => s.id === open?.id)
  const live = list?.filter((s) => s.active && s.productActive && !problems(s).length).length ?? 0
  return (
    <>
      <PageHead title="Print & Copy" sub={list ? `${live} on the till · ${list.length} in all` : 'Photocopy, print, laminate and bind jobs.'}
        actions={<>
          <Button asChild variant="secondary" className="h-10 px-4"><Link to="/admin/x/$key/$" params={{ key: 'documents', _splat: 'report' }}><BarChart3 size={16} /> Report</Link></Button>
          <Button className="h-10 px-4 font-bold" onClick={() => setCreating(true)}><Plus size={16} /> New print service</Button>
        </>} />
      {list && list.length === 0
        ? <EmptyState title="No print services yet">Add Photocopy, Print or Laminate, set a price for each size, colour and side — with bulk prices from 100 copies — and pick the paper each size uses.</EmptyState>
        : <div className="grid grid-cols-[minmax(0,1fr)] gap-3 md:grid-cols-2 xl:grid-cols-3">{list?.map((s) => <ServiceCard key={s.id} s={s} sym={sym} open={(what) => setOpen({ id: s.id, what })} />)}</div>}
      {creating && <NewService onClose={() => setCreating(false)} onMade={(id) => { setCreating(false); setOpen({ id, what: 'grid' }) }} />}
      {cur && open?.what === 'grid' && <GridSheet key={cur.id} s={cur} onClose={() => setOpen(null)} />}
      {cur && open?.what === 'recipe' && <RecipeEditor key={cur.id} id={cur.productId} onClose={() => setOpen(null)} />}
      {cur && open?.what === 'edit' && <ProductForm key={cur.id} editing={{ id: cur.productId }} onEdit={() => {}} onClose={() => setOpen(null)} />}
    </>
  )
}
