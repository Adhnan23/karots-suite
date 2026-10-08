import { Link, useNavigate, useRouterState } from '@tanstack/react-router'
import { KeyRound, LayoutDashboard, LockIcon, LogOut, Search, Store } from 'lucide-react'
import { type ReactNode, useEffect, useState } from 'react'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import type { Me, TillConfig } from '@/api/types.gen'
import { ModeToggle } from './ModeToggle'
import { CommandPalette } from './CommandPalette'
import { TillKeys } from './TillKeys'
import { TillTabs } from './TillTabs'
import { tillTabList } from './tillTabList'

const TOP_BTN = 'flex h-9 items-center gap-1.5 whitespace-nowrap rounded-[var(--radius-sm)] px-3 text-[13px] font-semibold text-muted-foreground hover:bg-muted hover:text-foreground'

function handleLock() {
  api('/auth/lock', { method: 'POST' }).finally(() => window.location.assign('/lock'))
}

/** Desktop/kiosk till frame (theme.md §7): 60px top bar + autofit body. `status` is
 * the drawer pill. Tabs: Sell now; M1b adds Returns, Credit, Receipts, … */
export function TillShell({ me, config, status, children }: { me: Me; config: TillConfig; status?: ReactNode; children: ReactNode }) {
  const navigate = useNavigate()
  const onSell = useRouterState({ select: (s) => s.location.pathname.replace(/\/$/, '') === '/till' })
  const [palette, setPalette] = useState(false)
  // Ctrl/Cmd K: jump to any till tab (karots-pos cashier palette).
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setPalette(true)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])
  const jumps = [
    ...tillTabList(config).map((t) => ({ key: t.to, label: t.label, to: t.to, icon: Store })),
    ...(config.isManager ? [{ key: 'admin', label: 'Admin', to: '/admin', icon: LayoutDashboard }] : []),
  ]
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
        <span className="flex min-w-0 max-w-[220px] items-center gap-2 truncate font-display text-[17px] font-bold whitespace-nowrap">
          <span className="grid h-[26px] w-[26px] shrink-0 place-items-center rounded-lg bg-brand text-sm font-extrabold text-brand-foreground shadow-[var(--glow)]" title={`${me.shopName} · ${me.user.name}`}>K</span>
          <span className="hidden truncate min-[1440px]:inline">{me.shopName}</span>
        </span>
        <span className="hidden whitespace-nowrap text-[13px] text-muted-foreground min-[1440px]:inline">{me.user.name}</span>
        {status}
        <TillTabs config={config} />
        <div className="ml-auto flex items-center gap-1 whitespace-nowrap">
          {onSell && <TillKeys />}
          <button type="button" onClick={() => setPalette(true)} className={TOP_BTN} aria-label="Search / jump (Ctrl K)" title="Search / jump (Ctrl K)">
            <Search size={16} />
          </button>
          <ModeToggle />
          {config.isManager && (
            <Link to="/admin" className={TOP_BTN} aria-label="Admin" title="Admin">
              <LayoutDashboard size={16} /> <span className="hidden min-[1440px]:inline">Admin</span>
            </Link>
          )}
          {config.canChangePin && (
            <Link to="/account/pin" className={TOP_BTN} aria-label="Change PIN" title="Change PIN">
              <KeyRound size={16} /> <span className="hidden min-[1440px]:inline">Change PIN</span>
            </Link>
          )}
          <button type="button" onClick={handleLock} className="flex h-9 items-center gap-1.5 whitespace-nowrap rounded-[var(--radius-sm)] border border-border bg-muted px-3 text-[13px] font-semibold">
            <LockIcon size={16} /> Lock
          </button>
          <button type="button" onClick={signOut} className={TOP_BTN} aria-label="Sign out" title="Sign out">
            <LogOut size={16} /> <span className="hidden min-[1440px]:inline">Sign out</span>
          </button>
        </div>
      </header>
      <div className="relative flex-1 overflow-hidden">{children}</div>
      <CommandPalette open={palette} onOpenChange={setPalette} entries={jumps} />
    </div>
  )
}
