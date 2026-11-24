import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { BookOpenCheck, Check, LoaderCircle, X } from 'lucide-react'
import { type ComponentProps, useState } from 'react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { useDebounced } from '@/lib/useDebounced'
import type { ProductAssist } from '../slots'
import { type Pick, searchQuery, statusQuery } from './api'

type Props = ComponentProps<ProductAssist['Assist']>

/** From catalogue, under the name on the product form, intake, the till's quick item and a delivery's new product
 * line: search the shop's copy, and a pick fills the name, unit and the shop's own fields (CTL-6). The person still
 * saves; the pick is linked and waits on Items to review (CTL-7). Without a copy it says so and they type (CTL-4). */
export function FromCatalogue({ name, where, fill }: Props) {
  const [open, setOpen] = useState(false)
  const [q, setQ] = useState('')
  const [picked, setPicked] = useState<Pick | null>(null)
  const compact = where === 'quick' || where === 'purchase'

  function start() {
    setQ(name.trim())
    setOpen(true)
  }
  function use(p: Pick) {
    fill({ name: p.name, unitId: p.unitId || undefined, extra: { itemId: p.id }, ...(p.sections && { sections: p.sections }) })
    setPicked(p)
    setOpen(false)
  }

  if (picked && !open) {
    return (
      <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[12px] text-muted-foreground">
        <BookOpenCheck size={13} className="text-brand" /> From the catalogue: <b className="text-foreground">{picked.name}</b>
        <button type="button" className="font-semibold text-brand underline-offset-2 hover:underline" onClick={start}>Pick again</button>
      </p>
    )
  }
  if (!open) {
    return (
      <div className="flex items-center gap-2">
        <Button type="button" variant="outline" onClick={start}
          className="h-8 gap-1.5 rounded-pill border-[color-mix(in_srgb,var(--brand)_40%,var(--border))] px-3 text-[13px] font-semibold">
          <BookOpenCheck size={14} className="text-brand" /> From catalogue
        </Button>
        {!compact && <small className="text-[12px] text-faint">Pick it from the Karots catalogue: the name and your fields fill in.</small>}
      </div>
    )
  }
  return (
    <section aria-label="From catalogue" className="flex flex-col gap-2 rounded-[var(--radius)] border border-[color-mix(in_srgb,var(--brand)_35%,var(--border))] bg-[color-mix(in_srgb,var(--brand)_5%,var(--card))] p-3">
      <header className="flex items-center gap-2">
        <BookOpenCheck size={15} className="text-brand" />
        <input autoFocus aria-label="Search the catalogue" placeholder="Search the catalogue: name, generic, company…" value={q} onChange={(e) => setQ(e.target.value)}
          name="catalogue-search" autoComplete="off"
          className="h-9 min-w-0 flex-1 rounded-[var(--radius-sm)] border border-border bg-background px-2.5 text-[13px] outline-none focus:border-brand" />
        <button type="button" aria-label="Close the catalogue" onClick={() => setOpen(false)} className="grid h-7 w-7 place-items-center rounded-full text-muted-foreground hover:bg-muted"><X size={15} /></button>
      </header>
      <Results q={q} onUse={use} />
    </section>
  )
}

function Results({ q, onUse }: { q: string; onUse: (p: Pick) => void }) {
  const term = useDebounced(q.trim(), 200)
  const status = useQuery(statusQuery)
  const res = useQuery({ ...searchQuery(term), enabled: term.length > 0, placeholderData: keepPreviousData })
  const say = (t: string) => <p className="text-[13px] text-muted-foreground">{t}</p>

  if (status.isError || res.isError) return say('The catalogue can’t be reached from here: type the product in.')
  if (status.data && status.data.count === 0) return say('The catalogue isn’t on this PC yet: type the product in.')
  if (!term) return say('Type a name, a generic or a company.')
  if (!res.data) return <LoaderCircle size={16} className="animate-spin text-muted-foreground" />
  if (res.data.length === 0) return say(`Nothing in the catalogue for “${term}”: type it in.`)
  return (
    <ul className={cn('flex max-h-72 flex-col gap-1.5 overflow-y-auto', res.isPlaceholderData && 'opacity-60')}>
      {res.data.map((p) => (
        <li key={p.id} className="flex flex-wrap items-center gap-2 rounded-[var(--radius-sm)] border border-border bg-card px-2.5 py-2">
          <span className="flex min-w-0 flex-1 flex-col">
            <b className="text-[13px] leading-snug">{p.name}</b>
            <small className="text-[12px] text-muted-foreground [overflow-wrap:anywhere]">{[p.unit, ...Object.values(p.fields ?? {})].filter(Boolean).join(' · ')}</small>
            {p.have && <small className="text-[12px] font-semibold text-warning">You have it: {p.have.name}</small>}
          </span>
          <Button type="button" size="sm" variant="outline" className="h-8 gap-1 px-3" onClick={() => onUse(p)}><Check size={14} /> Use this</Button>
        </li>
      ))}
    </ul>
  )
}
