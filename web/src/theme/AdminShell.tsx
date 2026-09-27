import { Link } from '@tanstack/react-router'
import { LogOut, Menu, PinIcon, UnlockIcon } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet'
import type { Me } from '@/api/types.gen'
import { ModeToggle } from './ModeToggle'
import { adminNav } from './nav'

function handleLock() {
  api('/auth/lock', { method: 'POST' }).finally(() => window.location.assign('/lock'))
}

function handleSignOut() {
  api('/auth/logout', { method: 'POST' })
    .then(() => window.location.assign('/login'))
    .catch((err) => {
      if (err instanceof ApiError && err.code === 'TILL_OPEN') toast.error(err.message)
      else window.location.assign('/login')
    })
}

function UserMenu({ me }: { me: Me }) {
  return (
    <>
      <Button variant="outline" size="sm" onClick={handleLock}>
        <UnlockIcon /> Lock
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline">{me.user.name}</Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem asChild>
            <Link to="/account/pin">
              <PinIcon /> Change PIN
            </Link>
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={handleSignOut} variant="destructive">
            <LogOut /> Sign out
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </>
  )
}

export function AdminShell({ me, children }: { me: Me; children: ReactNode }) {
  const nav = adminNav.filter((n) => !n.systemOnly || me.flags.isSystem)
  const mobileNav = nav.filter((n) => n.mobile)
  const moreNav = nav.filter((n) => !n.mobile)
  const [moreOpen, setMoreOpen] = useState(false)

  return (
    <div className="min-h-screen bg-background text-foreground md:flex">
      <nav
        aria-label="Sidebar"
        className="sticky top-0 hidden h-screen shrink-0 flex-col gap-1 border-r border-border bg-card p-3 md:flex md:w-[72px] lg:w-[232px]"
      >
        <div className="mb-4 truncate px-1 font-display text-lg font-semibold">
          <span className="lg:hidden">KS</span>
          <span className="hidden lg:inline">{me.shopName}</span>
        </div>
        {nav.map(({ to, label, icon: Icon }) => (
          <Link
            key={to}
            to={to}
            title={label}
            className="flex items-center gap-2 rounded-lg px-2 py-2 text-sm text-muted-foreground hover:bg-muted hover:text-foreground [&.active]:bg-brand-soft [&.active]:text-foreground"
            activeProps={{ className: 'active' }}
          >
            <Icon size={18} />
            <span className="hidden lg:inline">{label}</span>
          </Link>
        ))}
        <Link
          to="/till"
          title="Till"
          className="mt-auto flex items-center gap-2 rounded-lg bg-brand px-2 py-2 text-sm font-medium text-brand-foreground"
        >
          <span className="hidden lg:inline">Open the till</span>
          <span className="lg:hidden">Till</span>
        </Link>
      </nav>

      <div className="flex flex-1 flex-col">
        <header className="flex items-center justify-between border-b border-border bg-card px-4 py-3">
          <span className="truncate font-display font-semibold md:hidden">{me.shopName}</span>
          <span className="hidden font-display text-lg font-semibold md:inline" />
          <div className="ml-auto flex items-center gap-2">
            <ModeToggle />
            <UserMenu me={me} />
          </div>
        </header>
        <main className="flex-1 p-4 pb-20 md:pb-4">{children}</main>
      </div>

      <nav
        aria-label="Primary"
        className="fixed inset-x-0 bottom-0 flex border-t border-border bg-card md:hidden"
      >
        {mobileNav.map(({ to, label, icon: Icon }) => (
          <Link
            key={to}
            to={to}
            className="flex flex-1 flex-col items-center gap-0.5 py-2 text-xs text-muted-foreground [&.active]:text-brand"
            activeProps={{ className: 'active' }}
          >
            <Icon size={20} />
            {label}
          </Link>
        ))}
        {moreNav.length > 0 && (
          <Sheet open={moreOpen} onOpenChange={setMoreOpen}>
            <SheetTrigger asChild>
              <button type="button" className="flex flex-1 flex-col items-center gap-0.5 py-2 text-xs text-muted-foreground">
                <Menu size={20} />
                More
              </button>
            </SheetTrigger>
            <SheetContent side="bottom">
              <SheetHeader>
                <SheetTitle>More</SheetTitle>
              </SheetHeader>
              <div className="flex flex-col gap-1 p-4 pt-0">
                {moreNav.map(({ to, label, icon: Icon }) => (
                  <Link
                    key={to}
                    to={to}
                    onClick={() => setMoreOpen(false)}
                    className="flex items-center gap-2 rounded-lg px-2 py-2 text-sm hover:bg-muted"
                  >
                    <Icon size={18} /> {label}
                  </Link>
                ))}
              </div>
            </SheetContent>
          </Sheet>
        )}
      </nav>
    </div>
  )
}
