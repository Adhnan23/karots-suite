import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { Download, Plus } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { errMsg } from '@/features/till/api'
import { qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { createGroup, groupsQuery } from './api'
import { GroupPage } from './GroupPage'
import { groupsCsv, hasTiers } from './labels'
import type { Group, Tier } from './types'

const CHIP = 'h-9 rounded-pill border border-border px-3.5 text-[13px] font-semibold text-muted-foreground aria-pressed:border-transparent aria-pressed:bg-selected aria-pressed:text-selected-foreground'

function Level({ t }: { t: Tier }) {
  return (
    <span className="flex shrink-0 items-center gap-2 tabular-nums">
      <b>{qtyText(t.total)}</b>
      <span className="text-[12px] text-faint">{t.reorderLevel > 0 ? `reorder at ${t.reorderLevel}` : 'not tracked'}</span>
      {t.low && <Tag tone="danger">Low</Tag>}
    </span>
  )
}

function GroupCard({ g }: { g: Group }) {
  const tiers = g.tiers.filter((t) => t.active)
  return (
    <Link to="/admin/x/$key/$" params={{ key: 'alternatives', _splat: String(g.id) }}
      className={cn(SURFACE, 'flex min-w-0 flex-col gap-3 p-4 transition-colors hover:border-brand', !g.active && 'opacity-70')}>
      <header className="flex items-start justify-between gap-3">
        <h2 className="min-w-0 break-words font-display text-[17px] font-bold leading-snug">{g.name}</h2>
        <span className="flex shrink-0 items-center gap-1.5">
          {!g.active && <Tag tone="muted">Off</Tag>}
          <Tag tone="brand">{`${g.products} product${g.products === 1 ? '' : 's'}`}</Tag>
        </span>
      </header>
      {hasTiers(g) ? (
        <ul className="flex flex-col gap-1 text-[13.5px]">
          {tiers.map((t) => (
            <li key={t.id} className="flex items-center justify-between gap-3">
              <span className="min-w-0 truncate font-semibold">{t.name}</span>
              <Level t={t} />
            </li>
          ))}
        </ul>
      ) : tiers[0] && (
        <div className="flex items-center justify-between gap-3 text-[13.5px]"><span className="text-muted-foreground">In stock</span><Level t={tiers[0]} /></div>
      )}
    </Link>
  )
}

function download(gs: Group[]) {
  const a = document.createElement('a')
  a.href = URL.createObjectURL(new Blob([groupsCsv(gs)], { type: 'text/csv' }))
  a.download = 'alternatives.csv'
  a.click()
  URL.revokeObjectURL(a.href)
}

function NewGroup({ onClose }: { onClose: () => void }) {
  const [name, setName] = useState('')
  const [busy, setBusy] = useState(false)
  const navigate = useNavigate()
  const submit = async () => {
    setBusy(true)
    try {
      const g = await createGroup(name)
      toast.success(`${g.name} added`)
      onClose()
      void navigate({ to: '/admin/x/$key/$', params: { key: 'alternatives', _splat: String(g.id) } })
    } catch (e) {
      toast.error(errMsg(e))
      setBusy(false)
    }
  }
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title="New group" submit="Add group" busy={busy} onSubmit={() => void submit()}>
      <Field label="Name" hint="What the products are, e.g. USB flash drive 32GB. Add quality tiers later if you want them.">
        <input required autoFocus value={name} onChange={(e) => setName(e.target.value)} placeholder="USB flash drive 32GB" className={INPUT} />
      </Field>
    </FormSheet>
  )
}

/** Apps → Alternatives: interchangeable products. The sub-path is a group's id. */
export function AlternativesPage({ path }: { path: string }) {
  const [disabled, setDisabled] = useState(false)
  const [lowOnly, setLowOnly] = useState(false)
  const [adding, setAdding] = useState(false)
  const list = useQuery(groupsQuery(disabled)).data
  if (path) return <GroupPage id={Number(path)} />
  const low = list?.filter((g) => g.active && g.tiers.some((t) => t.active && t.low)) ?? []
  const shown = lowOnly ? low : list
  return (
    <>
      <PageHead title="Alternatives"
        sub={list ? `${list.length} group${list.length === 1 ? '' : 's'} · ${low.length} low` : 'Products that do the same job.'}
        actions={<>
          <Button variant="secondary" className="h-10 px-4" disabled={!list?.length} onClick={() => list && download(list)}><Download size={16} /> CSV</Button>
          <Button className="h-10 px-4 font-bold" onClick={() => setAdding(true)}><Plus size={16} /> New group</Button>
        </>} />
      <div className="mb-3 flex flex-wrap gap-2">
        <button type="button" aria-pressed={lowOnly} className={CHIP} onClick={() => setLowOnly(!lowOnly)}>Low only</button>
        <button type="button" aria-pressed={disabled} className={CHIP} onClick={() => setDisabled(!disabled)}>Show turned off</button>
      </div>
      {shown && shown.length === 0
        ? <EmptyState title={lowOnly ? 'Nothing is low 🎉' : 'No groups yet'}>
            {!lowOnly && 'Group products that do the same job. The till then finds the whole group, offers an in-stock swap when an item runs out, and Reorder judges stock per group.'}
          </EmptyState>
        : <div className="grid grid-cols-[minmax(0,1fr)] gap-3 md:grid-cols-2 xl:grid-cols-3">{shown?.map((g) => <GroupCard key={g.id} g={g} />)}</div>}
      {adding && <NewGroup onClose={() => setAdding(false)} />}
    </>
  )
}
