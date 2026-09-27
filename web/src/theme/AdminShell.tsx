import { Link } from '@tanstack/react-router'
import { LogOut, PinIcon, Search, UnlockIcon } from 'lucide-react'
import { useEffect, useRef, useState, type ReactNode } from 'react'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import type { Me } from '@/api/types.gen'
import { CommandPalette } from './CommandPalette'
import { Dock } from './Dock'
import { ModeToggle } from './ModeToggle'
import { adminNav } from './nav'
import { PhoneTabs } from './PhoneTabs'
import { Shelf } from './Shelf'

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

/** Tapping the user's name 8 times in the avatar menu opens the hidden system
 * menu (theme.md §6), same gesture as karots-pos. System user only. */
function useHiddenSystemMenu(isSystem: boolean) {
  const taps = useRef({ n: 0, at: 0 })
  return () => {
    if (!isSystem) return
    const now = Date.now()
    // Taps must be quick (<1.5s apart), as in karots-pos, so stray clicks over a session never add up.
    taps.current = { n: now - taps.current.at < 1500 ? taps.current.n + 1 : 1, at: now }
    if (taps.current.n >= 8) window.location.assign('/admin/system/appearance')
  }
}

function AvatarMenu({ me }: { me: Me }) {
  const tapName = useHiddenSystemMenu(me.flags.isSystem)
  const initials = me.user.name.slice(0, 1).toUpperCase()

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label="Account"
          className="grid h-9 w-9 place-items-center rounded-full bg-brand-soft font-bold text-foreground"
        >
          {initials}
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onSelect={(e) => { e.preventDefault(); tapName() }}>{me.user.name}</DropdownMenuItem>
        <div className="flex justify-center px-1.5 py-1 md:hidden">
          <ModeToggle />
        </div>
        <DropdownMenuSeparator />
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
  )
}

/** The dock admin shell (theme.md §6): a top bar, the floating dock + shelf on
 * desktop/tablet, a floating tab bar on the phone. No sidebar. */
export function AdminShell({ me, children }: { me: Me; children: ReactNode }) {
  const [shelf, setShelf] = useState<string | null>(null)
  const [paletteOpen, setPaletteOpen] = useState(false)

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setPaletteOpen((v) => !v)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const shelfArea = shelf && shelf !== 'apps' ? adminNav.find((a) => a.key === shelf) : undefined

  return (
    <div className="min-h-screen text-foreground">
      <header className="flex items-center gap-2.5 px-4 pt-4 md:gap-3 md:px-10">
        <span className="hidden h-[30px] w-[30px] flex-none place-items-center rounded-[9px] bg-brand font-display text-sm font-extrabold text-brand-foreground md:grid">
          K
        </span>
        <span className="min-w-0 flex-1 truncate font-display text-base font-bold md:flex-none md:text-lg">{me.shopName}</span>
        <div className="ml-auto flex flex-none items-center gap-1.5 md:gap-2.5">
          <button
            type="button"
            onClick={() => setPaletteOpen(true)}
            className="hidden min-w-[220px] items-center gap-2 rounded-full border border-border bg-muted px-3 py-2 text-sm text-muted-foreground md:flex"
          >
            <Search size={16} />
            <span className="flex-1 text-left">Search pages, products, bills</span>
            <kbd className="rounded border border-border bg-card px-1.5 py-0.5 text-[11px] font-semibold">Ctrl K</kbd>
          </button>
          <div className="hidden md:block">
            <ModeToggle />
          </div>
          <button
            type="button"
            onClick={handleLock}
            aria-label="Lock"
            className="flex h-9 items-center gap-1.5 rounded-full border border-border bg-muted px-2.5 text-sm font-semibold md:px-3"
          >
            <UnlockIcon size={16} />
            <span className="hidden md:inline">Lock</span>
          </button>
          <AvatarMenu me={me} />
        </div>
      </header>

      <main className="p-4 pb-[100px] md:px-10 md:pt-5 md:pb-[110px]">{children}</main>

      <Dock current={shelf} onToggle={setShelf} />
      {shelf === 'apps' && <Shelf kind="apps" onClose={() => setShelf(null)} />}
      {shelfArea && <Shelf kind="area" area={shelfArea} onClose={() => setShelf(null)} />}
      <PhoneTabs />
      <CommandPalette open={paletteOpen} onOpenChange={setPaletteOpen} />
    </div>
  )
}
