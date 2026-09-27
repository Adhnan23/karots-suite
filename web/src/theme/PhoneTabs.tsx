import { Link } from '@tanstack/react-router'
import { Home, Menu, Store } from 'lucide-react'
import { useState } from 'react'
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { adminNav, apps } from './nav'

/** Phone bottom tab bar (theme.md §6): Home, Till centred and accented, More.
 * More holds any ported areas and plugin apps — none exist yet in M0. */
export function PhoneTabs() {
  const [open, setOpen] = useState(false)
  const areas = adminNav.filter((a) => a.pages.length > 0)
  const hasMore = areas.length > 0 || apps.length > 0

  return (
    <>
      <nav
        aria-label="Primary"
        className="fixed inset-x-2.5 bottom-2.5 z-20 grid grid-cols-3 rounded-[22px] border border-border bg-bar p-2 shadow-[var(--shadow-float)] [backdrop-filter:var(--glass)] md:hidden"
      >
        <Link
          to="/admin"
          className="flex flex-col items-center gap-0.5 rounded-2xl py-1.5 text-[11px] font-bold text-faint [&.active]:bg-brand-soft [&.active]:text-foreground"
          activeProps={{ className: 'active' }}
        >
          <Home size={20} />
          Home
        </Link>
        <Link
          to="/till"
          className="flex flex-col items-center gap-0.5 rounded-2xl bg-brand py-1.5 text-[11px] font-bold text-brand-foreground shadow-[var(--glow)]"
        >
          <Store size={20} />
          Till
        </Link>
        <button
          type="button"
          onClick={() => setOpen(true)}
          disabled={!hasMore}
          className="flex flex-col items-center gap-0.5 rounded-2xl py-1.5 text-[11px] font-bold text-faint disabled:opacity-40"
        >
          <Menu size={20} />
          More
        </button>
      </nav>
      <Sheet open={open} onOpenChange={setOpen}>
        <SheetContent side="bottom">
          <SheetHeader>
            <SheetTitle>More</SheetTitle>
          </SheetHeader>
          <div className="flex flex-col gap-1 p-4 pt-0">
            {!hasMore && <p className="text-sm text-muted-foreground">More areas arrive in later milestones.</p>}
            {areas.map((a) => (
              <div key={a.key} className="flex items-center gap-2 rounded-lg px-2 py-2 text-sm text-muted-foreground">
                <a.icon size={18} />
                {a.label}
              </div>
            ))}
          </div>
        </SheetContent>
      </Sheet>
    </>
  )
}
