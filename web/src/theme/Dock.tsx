import { Link } from '@tanstack/react-router'
import { Home, LayoutGrid, Store } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { adminNav } from './nav'

function DockButton({
  icon: Icon,
  label,
  active,
  onClick,
}: {
  icon: LucideIcon
  label: string
  active: boolean
  onClick: () => void
}) {
  return (
    <button
      type="button"
      aria-label={label}
      aria-pressed={active}
      onClick={onClick}
      className="group relative grid h-[54px] w-[54px] place-items-center rounded-2xl text-muted-foreground transition-[width,height,background] hover:h-[62px] hover:w-[62px] hover:bg-muted hover:text-foreground aria-pressed:bg-selected aria-pressed:text-selected-foreground"
    >
      <Icon size={22} />
      <span className="pointer-events-none absolute bottom-full mb-2.5 rounded-lg bg-foreground px-2.5 py-1 text-xs font-bold whitespace-nowrap text-background opacity-0 transition-opacity group-hover:opacity-100">
        {label}
      </span>
    </button>
  )
}

/** The floating dock (theme.md §6): Home, ported areas, a divider, Apps, then
 * Open the till. Areas with no pages yet (all of them in M0) are left out. */
export function Dock({ current, onToggle }: { current: string | null; onToggle: (key: string | null) => void }) {
  const areas = adminNav.filter((a) => a.pages.length > 0)

  return (
    <nav
      aria-label="Main"
      className="fixed bottom-[18px] left-1/2 z-20 flex -translate-x-1/2 items-end gap-1.5 rounded-3xl border border-border bg-bar p-2 shadow-[var(--shadow-float)] [backdrop-filter:var(--glass)]"
    >
      <Link to="/admin" aria-label="Home" className="grid h-[54px] w-[54px] place-items-center rounded-2xl text-muted-foreground hover:bg-muted hover:text-foreground">
        <Home size={22} />
      </Link>
      {areas.length > 0 && <span className="mx-1 w-px self-stretch bg-border" />}
      {areas.map((a) => (
        <DockButton
          key={a.key}
          icon={a.icon}
          label={a.label}
          active={current === a.key}
          onClick={() => onToggle(current === a.key ? null : a.key)}
        />
      ))}
      <span className="mx-1 w-px self-stretch bg-border" />
      <DockButton icon={LayoutGrid} label="Apps" active={current === 'apps'} onClick={() => onToggle(current === 'apps' ? null : 'apps')} />
      <Link
        to="/till"
        aria-label="Open the till"
        className="grid h-[54px] w-[54px] place-items-center rounded-2xl bg-brand text-brand-foreground shadow-[var(--glow)]"
      >
        <Store size={22} />
      </Link>
    </nav>
  )
}
