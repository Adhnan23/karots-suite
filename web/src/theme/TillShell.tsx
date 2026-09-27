import { LogOut, UnlockIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import type { Me } from '@/api/types.gen'
import { ModeToggle } from './ModeToggle'

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

/** Desktop/kiosk till frame (theme.md §7): 60px top bar + autofit body. No mobile layout.
 * M1 adds the drawer pill and hotkey hints between the name and the controls. */
export function TillShell({ me, children }: { me: Me; children: ReactNode }) {
  return (
    <div className="flex h-screen min-w-[1024px] flex-col text-foreground">
      <header className="flex h-[60px] shrink-0 items-center gap-4 border-b border-border bg-card px-[18px] [backdrop-filter:var(--glass)]">
        <span className="flex items-center gap-2 font-display text-[17px] font-bold">
          <span className="grid h-[26px] w-[26px] place-items-center rounded-lg bg-brand text-sm font-extrabold text-brand-foreground shadow-[var(--glow)]">
            K
          </span>
          {me.shopName}
        </span>
        <span className="text-[13px] text-muted-foreground">{me.user.name}</span>
        <div className="ml-auto flex items-center gap-2">
          <ModeToggle />
          <button
            type="button"
            onClick={handleLock}
            className="flex h-9 items-center gap-1.5 rounded-[var(--radius-sm)] border border-border bg-muted px-3 text-[13px] font-semibold"
          >
            <UnlockIcon size={16} /> Lock
          </button>
          <button
            type="button"
            onClick={handleSignOut}
            className="flex h-9 items-center gap-1.5 rounded-[var(--radius-sm)] px-3 text-[13px] font-semibold text-muted-foreground hover:bg-muted hover:text-foreground"
          >
            <LogOut size={16} /> Sign out
          </button>
        </div>
      </header>
      <div className="flex-1 overflow-auto">{children}</div>
    </div>
  )
}
