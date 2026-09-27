import { LogOut, UnlockIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
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

/** Desktop/kiosk till frame (theme.md §6): 60px top bar + autofit body. No mobile layout. */
export function TillShell({ me, children }: { me: Me; children: ReactNode }) {
  return (
    <div className="flex h-screen min-w-[1024px] flex-col bg-background text-foreground">
      <header className="flex h-[60px] shrink-0 items-center gap-4 border-b border-border bg-card px-4">
        <span className="font-display text-lg font-semibold">{me.shopName}</span>
        <span className="text-sm text-muted-foreground">{me.user.name}</span>
        <div className="ml-auto flex items-center gap-2">
          <ModeToggle />
          <Button variant="outline" size="sm" onClick={handleLock}>
            <UnlockIcon /> Lock
          </Button>
          <Button variant="ghost" size="sm" onClick={handleSignOut}>
            <LogOut /> Sign out
          </Button>
        </div>
      </header>
      <div className="flex-1 overflow-auto">{children}</div>
    </div>
  )
}
