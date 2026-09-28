import { Link, useNavigate } from '@tanstack/react-router'
import { KeyRound, LayoutDashboard, LockIcon, LogOut } from 'lucide-react'
import type { ReactNode } from 'react'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import type { Me, TillConfig } from '@/api/types.gen'
import { Kbd } from '@/features/till/bits'
import { ModeToggle } from './ModeToggle'

const TOP_BTN = 'flex h-9 items-center gap-1.5 rounded-[var(--radius-sm)] px-3 text-[13px] font-semibold text-muted-foreground hover:bg-muted hover:text-foreground'

function handleLock() {
  api('/auth/lock', { method: 'POST' }).finally(() => window.location.assign('/lock'))
}

/** Desktop/kiosk till frame (theme.md §7): 60px top bar + autofit body. `status` is
 * the drawer pill. Tabs: Sell now; M1b adds Returns, Credit, Receipts, … */
export function TillShell({ me, config, status, children }: { me: Me; config: TillConfig; status?: ReactNode; children: ReactNode }) {
  const navigate = useNavigate()
  function signOut() {
    api('/auth/logout', { method: 'POST' })
      .then(() => window.location.assign('/login'))
      .catch((err) => {
        if (err instanceof ApiError && err.code === 'TILL_OPEN') {
          toast.error(err.message)
          void navigate({ to: '/till', search: { logout: true } })
        } else window.location.assign('/login')
      })
  }
  return (
    <div className="flex h-screen min-w-[1024px] flex-col text-foreground">
      <header className="flex h-[60px] shrink-0 items-center gap-4 border-b border-border bg-card px-[18px] [backdrop-filter:var(--glass)]">
        <span className="flex items-center gap-2 font-display text-[17px] font-bold">
          <span className="grid h-[26px] w-[26px] place-items-center rounded-lg bg-brand text-sm font-extrabold text-brand-foreground shadow-[var(--glow)]">K</span>
          {me.shopName}
        </span>
        <span className="text-[13px] text-muted-foreground">{me.user.name}</span>
        {status}
        <nav aria-label="Till sections" className="flex items-center gap-1 text-[13px] font-semibold">
          <Link to="/till" activeOptions={{ exact: true, includeSearch: false }}
            className="rounded-full px-3 py-1.5 text-muted-foreground hover:bg-muted data-[status=active]:bg-selected data-[status=active]:text-selected-foreground">
            Sell
          </Link>
        </nav>
        <div className="ml-auto hidden items-center gap-2.5 text-[12px] text-muted-foreground min-[1200px]:flex" aria-label="Keyboard shortcuts">
          <Kbd>F1</Kbd>Menu <Kbd>F2</Kbd>Search <Kbd>F4</Kbd>Hold <Kbd>F9</Kbd>Discount <Kbd>F10</Kbd>Pay
        </div>
        <div className="ml-auto flex items-center gap-1 min-[1200px]:ml-0">
          <ModeToggle />
          {config.isManager && <Link to="/admin" className={TOP_BTN}><LayoutDashboard size={16} /> Admin</Link>}
          {config.canChangePin && <Link to="/account/pin" className={TOP_BTN}><KeyRound size={16} /> Change PIN</Link>}
          <button type="button" onClick={handleLock} className="flex h-9 items-center gap-1.5 rounded-[var(--radius-sm)] border border-border bg-muted px-3 text-[13px] font-semibold">
            <LockIcon size={16} /> Lock
          </button>
          <button type="button" onClick={signOut} className={TOP_BTN}><LogOut size={16} /> Sign out</button>
        </div>
      </header>
      <div className="relative flex-1 overflow-hidden">{children}</div>
    </div>
  )
}
