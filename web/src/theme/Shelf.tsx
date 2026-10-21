import { Link, useRouterState } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { cn } from '@/lib/utils'
import type { NavArea, NavPage } from './nav'
import { apps, whereIs } from './nav'

type Props =
  | { kind: 'area'; area: NavArea; onClose: () => void }
  | { kind: 'apps'; onClose: () => void }

/** The panel a dock click raises above it (theme.md §6). Closes on Escape or a
 * click outside. Apps is the plugin slot — empty until M7. */
export function Shelf(props: Props) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') props.onClose()
    }
    const onClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) props.onClose()
    }
    document.addEventListener('keydown', onKey)
    document.addEventListener('mousedown', onClick)
    return () => {
      document.removeEventListener('keydown', onKey)
      document.removeEventListener('mousedown', onClick)
    }
  }, [props])

  const label = props.kind === 'apps' ? 'Apps' : props.area.label

  return (
    <div
      ref={ref}
      role="dialog"
      aria-label={label}
      className="fixed bottom-[104px] left-1/2 z-20 w-[min(880px,calc(100vw-32px))] -translate-x-1/2 rounded-xl border border-border bg-popover p-3 shadow-[var(--shadow-float)]"
    >
      {props.kind === 'apps' ? (
        apps.length === 0 ? (
          <p className="p-8 text-center text-sm text-muted-foreground">No apps installed on this shop yet.</p>
        ) : (
          <div className="grid grid-cols-4 gap-2.5">
            {apps.map((a) => (
              <Link key={a.key} to={a.to} onClick={props.onClose} className="flex flex-col gap-2.5 rounded-2xl border border-border bg-card p-3.5 text-left transition-colors hover:border-brand">
                <span className="grid h-11 w-11 place-items-center rounded-[var(--radius-sm)] text-brand-foreground" style={{ background: a.color }}>
                  <a.icon size={20} />
                </span>
                <b className="text-sm">{a.label}</b>
                <span className="text-xs text-muted-foreground">{a.description}</span>
              </Link>
            ))}
          </div>
        )
      ) : (
        <AreaShelf area={props.area} onClose={props.onClose} />
      )}
    </div>
  )
}

/** An area's shelf (theme.md §6): a hero lead card beside the pages as icon
 * tiles. Descriptions live in the lead card, one at a time — the page pointed at
 * (or focused) — so the grid stays names only. */
function AreaShelf({ area, onClose }: { area: NavArea; onClose: () => void }) {
  const path = useRouterState({ select: (s) => s.location.pathname })
  const current = whereIs([area], path)?.page.to
  const [hover, setHover] = useState<NavPage | null>(null)
  const many = area.pages.length > 9
  return (
    <div className="grid grid-cols-[220px_1fr] gap-4">
      <div className="relative flex flex-col justify-between gap-6 overflow-hidden rounded-[var(--radius)] bg-hero bg-[image:var(--hero-art)] p-5 text-hero-foreground">
        <div className="flex items-center gap-2.5">
          <span className="grid h-10 w-10 place-items-center rounded-[var(--radius-sm)] bg-hero-foreground/10"><area.icon size={20} /></span>
          <span>
            <b className="block font-display text-lg leading-tight">{area.label}</b>
            <small className="text-hero-muted">{area.pages.length} pages</small>
          </span>
        </div>
        {/* Fixed height: a longer description must never resize the shelf. */}
        <div aria-live="polite" className="h-[96px] overflow-hidden">
          {hover ? (
            <>
              <b className="block truncate text-[15px]">{hover.label}</b>
              <p className="mt-1 line-clamp-3 text-[13px] leading-snug text-hero-muted">{hover.description}</p>
            </>
          ) : <p className="text-[13px] leading-snug text-hero-muted">{area.blurb}</p>}
        </div>
      </div>
      <div className={cn('grid grid-cols-3 content-start gap-x-1', many ? 'auto-rows-[46px]' : 'auto-rows-[56px]')} onMouseLeave={() => setHover(null)}>
        {area.pages.map((p) => {
          const here = p.to === current
          return (
            <Link key={p.to} to={p.to} onClick={onClose} aria-current={here ? 'page' : undefined}
              onMouseEnter={() => setHover(p)} onFocus={() => setHover(p)}
              className={cn('group flex min-w-0 items-center gap-3 rounded-xl outline-none', many ? 'px-2 py-1.5' : 'p-2')}>
              <span className={cn('relative grid flex-none place-items-center rounded-[var(--radius-sm)] transition-[transform,background-color,color,box-shadow] duration-(--dur-fast) group-hover:-translate-y-0.5 group-hover:bg-brand group-hover:text-brand-foreground group-hover:shadow-[var(--glow)] group-focus-visible:bg-brand group-focus-visible:text-brand-foreground group-focus-visible:ring-2 group-focus-visible:ring-ring',
                many ? 'h-8 w-8 rounded-[var(--radius-sm)]' : 'h-10 w-10', here ? 'bg-selected text-selected-foreground' : 'bg-muted text-muted-foreground')}>
                <p.icon size={many ? 16 : 19} />
                {p.badge ? <span className="absolute -top-1.5 -right-1.5 min-w-[18px] rounded-pill bg-danger px-1 text-center text-[11px] font-extrabold leading-[18px] text-danger-foreground">{p.badge}</span> : null}
              </span>
              <span className={cn('line-clamp-2 text-sm leading-tight font-semibold transition-colors group-hover:text-foreground', here ? 'text-foreground' : 'text-muted-foreground')}>{p.label}</span>
            </Link>
          )
        })}
      </div>
    </div>
  )
}
