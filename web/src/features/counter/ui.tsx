import type { UseInfiniteQueryResult } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { PANEL } from '@/features/till/ui'

/** A counter tab: two panels. flush = the left panel lays out its own padding (a find list). */
export function CounterPage({ children, aside, flush }: { children: ReactNode; aside: ReactNode; flush?: boolean }) {
  return (
    <div className="grid h-full min-h-0 grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)] gap-[14px] p-[14px]">
      <section className={cn(PANEL, 'flex min-h-0 flex-col', flush ? 'overflow-hidden' : 'overflow-auto p-5')}>{children}</section>
      <aside className={cn(PANEL, 'flex min-h-0 flex-col overflow-hidden')}>{aside}</aside>
    </div>
  )
}

/** The last row of a paged list (queries.ts paged): older rows are one tap away,
 *  so a list never stops silently at its newest page. */
export function MoreRows({ list }: { list: UseInfiniteQueryResult<unknown> }) {
  if (!list.hasNextPage) return null
  return (
    <li>
      <button type="button" disabled={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}
        className="w-full rounded-[var(--radius-sm)] px-3 py-2.5 text-[13px] font-semibold text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-60">
        {list.isFetchingNextPage ? 'Loading…' : 'Show older'}
      </button>
    </li>
  )
}

export function PaneTitle({ title, sub, right }: { title: string; sub?: ReactNode; right?: ReactNode }) {
  return (
    <header className="flex items-center justify-between gap-3 px-4 pb-2.5 pt-3.5">
      <span className="font-bold">{title}{sub && <small className="ml-2 font-medium text-muted-foreground">{sub}</small>}</span>
      {right}
    </header>
  )
}

/** Tap-to-pick tiles for "where does the cash come from". value "" = my drawer. */
export function SourcePicker({ value, onChange, lockers, sym }: {
  value: string
  onChange: (v: string) => void
  lockers: { id: number; name: string; balance: string }[]
  sym: string
}) {
  const opts = [{ v: '', name: 'My drawer', sub: 'cash in the till' }, ...lockers.map((l) => ({ v: String(l.id), name: l.name, sub: money(sym, l.balance) }))]
  return (
    <div role="radiogroup" aria-label="Paid from" className="grid grid-cols-[repeat(auto-fill,minmax(140px,1fr))] gap-1.5">
      {opts.map((o) => (
        <button key={o.v} type="button" role="radio" aria-checked={value === o.v} onClick={() => onChange(o.v)}
          className="flex flex-col items-start rounded-[var(--radius-sm)] border border-border bg-muted px-3 py-2 text-left aria-checked:border-brand aria-checked:bg-brand aria-checked:text-brand-foreground">
          <b className="text-[13px]">{o.name}</b>
          <small className="text-[11.5px] opacity-75 tabular-nums">{o.sub}</small>
        </button>
      ))}
    </div>
  )
}
