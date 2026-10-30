import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { hhmm, initials, LivePill, Pill } from '../bits'
import type { Who } from '../session'
import type { SyncState } from '../sync'

/** Live / PC-not-reachable, the drawer (or what waits to be sent), and who is on. */
export function TopBar({ who, sync, drawerOpen, onSwitch }: { who: Who; sync: SyncState; drawerOpen: boolean; onSwitch: () => void }) {
  return (
    <div className="flex items-center gap-2 px-3 pt-2 pb-2">
      <LivePill live={sync.live} />
      {sync.live || sync.waiting === 0
        ? <Pill>{drawerOpen ? 'Drawer open' : 'Drawer closed'}</Pill>
        : <Pill>Waiting <span aria-label={`${sync.waiting} waiting`} className="grid h-[18px] min-w-[18px] place-items-center rounded-pill bg-warning px-1.5 text-[10.5px] font-extrabold text-background">{sync.waiting}</span></Pill>}
      <DropdownMenu>
        <DropdownMenuTrigger aria-label={`${who.name} — menu`} className="ml-auto grid size-[30px] place-items-center rounded-full bg-brand text-xs font-bold text-brand-foreground">
          {initials(who.name)}
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <div className="px-2 py-1.5 text-sm"><b>{who.name}</b><br /><small className="text-muted-foreground">Copy of {hhmm(sync.copyAt)}</small></div>
          <DropdownMenuItem onSelect={onSwitch}>Switch person</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}

/** PC down: what the cashier must know, always on screen. */
export function OfflineBanner({ copyAt }: { copyAt: string | null }) {
  const old = !copyAt || Date.now() - Date.parse(copyAt) > 30 * 60_000
  return (
    <div role="note" className="mx-3 mb-2 rounded-[var(--radius-sm)] border border-[color-mix(in_srgb,var(--warning)_35%,transparent)] bg-[color-mix(in_srgb,var(--warning)_14%,transparent)] px-3 py-2 text-xs leading-snug">
      <b className="text-warning">Selling from the copy of {hhmm(copyAt)}.</b> Sales are kept on this phone and sent when the PC is back. No printing until then.
      {old && <b className="mt-1 block text-warning">The copy is over half an hour old — prices and stock may have changed.</b>}
    </div>
  )
}
