import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { BarChart3, Plus } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { configQuery, errMsg } from '@/features/till/api'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { combosQuery, deleteCombo, setActive } from './api'
import { ComboSheet } from './ComboSheet'
import { dealLabel, soldLabel } from './labels'
import { setPrice } from './pricing'
import { ComboReport } from './Report'
import type { Combo } from './types'

async function act(p: Promise<unknown>, done: string) {
  try {
    await p
    toast.success(done)
  } catch (e) {
    toast.error(errMsg(e))
  }
}

const dateText = (d: string) => new Date(`${d}T00:00`).toLocaleDateString(undefined, { day: 'numeric', month: 'short' })

function windowTag(c: Combo) {
  if (!c.active) return <Tag tone="muted">Off</Tag>
  if (c.window === 'scheduled') return <Tag tone="brand">{`Starts ${dateText(c.startsOn)}`}</Tag>
  if (c.window === 'ended') return <Tag tone="muted">{`Ended ${dateText(c.endsOn)}`}</Tag>
  if (c.endsOn) return <Tag tone="brand">{`On till ${dateText(c.endsOn)}`}</Tag>
  return null
}

function ComboCard({ c, sym, onEdit }: { c: Combo; sym: string; onEdit: () => void }) {
  const { full, price } = setPrice(c, 'retail')
  const sold = soldLabel(c, sym)
  return (
    <article className={cn(SURFACE, 'flex min-w-0 flex-col gap-3 p-4', (!c.active || c.window !== 'on') && 'opacity-75')}>
      <header className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="break-words font-display text-[17px] font-bold leading-snug">{c.name}</h2>
          <div className="mt-1 flex flex-wrap items-center gap-1.5">
            <Tag tone="brand">{dealLabel(c, sym)}</Tag>
            {windowTag(c)}
          </div>
        </div>
        <div className="shrink-0 text-right tabular-nums">
          <s className="block text-[12px] text-faint">{money(sym, full / 100)}</s>
          <b className="font-display text-[19px]">{money(sym, price / 100)}</b>
        </div>
      </header>
      <ul className="flex flex-col gap-0.5 text-[13.5px]">
        {c.members.map((m) => (
          <li key={m.productId} className="flex justify-between gap-3">
            <span className="min-w-0 truncate">{m.name} <span className="text-muted-foreground">× {Number(m.qty)}</span></span>
            <span className="shrink-0 text-muted-foreground tabular-nums">{money(sym, m.sellingPrice)}</span>
          </li>
        ))}
      </ul>
      {(c.problem || c.warnings.length > 0) && (
        <div className="flex flex-wrap gap-1.5">
          {c.problem && <Tag tone="danger">{`${c.problem} — can't be sold`}</Tag>}
          {c.warnings.map((w) => <Tag key={w} tone="warn">{w}</Tag>)}
        </div>
      )}
      <footer className="mt-auto flex flex-wrap items-center gap-2 border-t border-border pt-3">
        <span className="w-full text-[12.5px] text-muted-foreground xl:w-auto xl:min-w-0 xl:flex-1">{sold || 'Not sold yet'}</span>
        <span className="flex-1 xl:hidden" />
        <Button variant="ghost" className="h-9 px-3" onClick={() => void act(setActive(c.id, !c.active), c.active ? 'Combo turned off' : 'Combo turned on')}>
          {c.active ? 'Turn off' : 'Turn on'}
        </Button>
        <Button variant="secondary" className="h-9 px-3" onClick={onEdit}>Edit</Button>
        <ConfirmButton label="Delete" title={`Delete ${c.name}?`} confirm="Delete" className="h-9 px-3"
          body="It leaves the till. Its sales stay in the Combos report." onConfirm={() => act(deleteCombo(c.id), 'Combo deleted')} />
      </footer>
    </article>
  )
}

/** Apps → Combos: deals rung in one tap. The sub-path report is the Combos report. */
export function CombosPage({ path }: { path: string }) {
  const sym = useSuspenseQuery(configQuery).data.symbol
  const list = useQuery(combosQuery).data
  const [editing, setEditing] = useState<{ c: Combo | null; n: number } | null>(null)
  if (path === 'report') return <ComboReport />
  const on = list?.filter((c) => c.active && c.window === 'on' && !c.problem).length ?? 0
  return (
    <>
      <PageHead title="Combos" sub={list ? `${on} on the till · ${list.length} in all` : 'Product deals rung in one tap.'}
        actions={<>
          <Button asChild variant="secondary" className="h-10 px-4"><Link to="/admin/x/$key/$" params={{ key: 'combos', _splat: 'report' }}><BarChart3 size={16} /> Report</Link></Button>
          <Button className="h-10 px-4 font-bold" onClick={() => setEditing({ c: null, n: Date.now() })}><Plus size={16} /> New combo</Button>
        </>} />
      {list && list.length === 0
        ? <EmptyState title="No combos yet">Bundle products at a deal price — the till rings them as one set, and spots them when the items are scanned one by one.</EmptyState>
        : <div className="grid grid-cols-[minmax(0,1fr)] gap-3 md:grid-cols-2 xl:grid-cols-3">{list?.map((c) => <ComboCard key={c.id} c={c} sym={sym} onEdit={() => setEditing({ c, n: Date.now() })} />)}</div>}
      {editing && <ComboSheet key={editing.n} combo={editing.c} sym={sym} open onClose={() => setEditing(null)} />}
    </>
  )
}
