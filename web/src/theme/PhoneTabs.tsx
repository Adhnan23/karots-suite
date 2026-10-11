import { Link } from '@tanstack/react-router'
import { Home, LayoutGrid, Menu, Store } from 'lucide-react'
import { useState } from 'react'
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { apps, type NavArea } from './nav'

/** Phone bottom tab bar (theme.md §6): Home, Till centred and accented, More.
 * More holds every area's pages and the plugin apps. */
export function PhoneTabs({ areas }: { areas: NavArea[] }) {
  const [open, setOpen] = useState(false)
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
          activeOptions={{ exact: true }}
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
          <div className="flex max-h-[70vh] flex-col gap-4 overflow-auto p-4 pt-0">
            {areas.map((a) => (
              <section key={a.key}>
                <h3 className="mb-1.5 flex items-center gap-2 text-[12px] font-bold uppercase tracking-wide text-faint">
                  <a.icon size={15} />
                  {a.label}
                </h3>
                <div className="grid grid-cols-2 gap-1.5">
                  {a.pages.map((p) => (
                    <Link key={p.to} to={p.to} onClick={() => setOpen(false)}
                      className="flex min-w-0 items-center gap-2 rounded-[var(--radius-sm)] border border-border bg-card px-3 py-2.5 text-sm font-semibold">
                      <p.icon size={16} className="flex-none text-muted-foreground" />
                      <span className="truncate">{p.label}</span>
                    </Link>
                  ))}
                </div>
              </section>
            ))}
            {apps.length > 0 && (
              <section>
                <h3 className="mb-1.5 flex items-center gap-2 text-[12px] font-bold uppercase tracking-wide text-faint">
                  <LayoutGrid size={15} />
                  Apps
                </h3>
                <div className="grid grid-cols-2 gap-1.5">
                  {apps.map((a) => (
                    <Link key={a.key} to={a.to} onClick={() => setOpen(false)}
                      className="flex min-w-0 items-center gap-2 rounded-[var(--radius-sm)] border border-border bg-card px-3 py-2.5 text-sm font-semibold">
                      <a.icon size={16} className="flex-none text-muted-foreground" />
                      <span className="truncate">{a.label}</span>
                    </Link>
                  ))}
                </div>
              </section>
            )}
          </div>
        </SheetContent>
      </Sheet>
    </>
  )
}
