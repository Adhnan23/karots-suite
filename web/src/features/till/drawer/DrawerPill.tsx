import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { ChevronDown, Eye, EyeOff, Lock as LockIcon } from 'lucide-react'
import { useState } from 'react'
import { DropdownMenu, DropdownMenuContent, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, drawerQuery } from '../api'

const HIDE_KEY = 'ks-till-hide-cash'
const readHidden = () => {
  try { return localStorage.getItem(HIDE_KEY) === '1' } catch { return false }
}

/** Top-bar drawer chip: a dot and the cash in the drawer (or dots, when the
 * counter hides it from customers). Tap for the float, the drawer total and the
 * drawer moves — the actions open on the Sell screen (?drawer=…). */
export function DrawerPill() {
  const cfg = useQuery(configQuery).data
  const d = useQuery(drawerQuery).data
  const navigate = useNavigate()
  const [hidden, setHidden] = useState(readHidden)
  if (!cfg || !d) return null
  const s = d.summary.session
  const dot = <span className={cn('h-2 w-2 shrink-0 rounded-full', s ? 'bg-success shadow-[0_0_0_4px_color-mix(in_srgb,var(--success)_25%,transparent)]' : 'bg-faint')} />
  if (!s) {
    return <span className="flex items-center gap-2 whitespace-nowrap rounded-pill border border-border bg-muted px-3 py-1.5 text-[13px]">{dot} Till closed</span>
  }
  const cash = money(cfg.symbol, d.summary.expected)
  const toggleHidden = () => {
    const next = !hidden
    setHidden(next)
    try { localStorage.setItem(HIDE_KEY, next ? '1' : '0') } catch { /* private mode */ }
  }
  const go = (drawer: 'deposit' | 'withdraw' | 'close') => void navigate({ to: '/till', search: { drawer } })
  const opened = new Date(s.openedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button type="button" aria-label={`Till open, ${hidden ? 'amount hidden' : `${cash} in the drawer`}`}
          className="flex shrink-0 items-center gap-2 whitespace-nowrap rounded-pill border border-border bg-muted px-3 py-1.5 text-[13px] hover:border-selected">
          {dot}
          <b className="tabular-nums">{hidden ? `${cfg.symbol} ••••` : cash}</b>
          <ChevronDown size={14} className="text-faint" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-80 p-3">
        <p className="text-[12px] font-semibold text-muted-foreground">Till open since {opened}</p>
        <dl className="mt-2 grid grid-cols-[1fr_auto] gap-y-1 text-sm">
          <dt className="text-muted-foreground">Float</dt><dd className="text-right tabular-nums">{money(cfg.symbol, s.openingCash)}</dd>
          <dt className="text-muted-foreground">In drawer now</dt><dd className="text-right font-bold tabular-nums">{cash}</dd>
        </dl>
        <div className="mt-3 grid grid-cols-3 gap-1.5">
          <button type="button" onClick={() => go('deposit')} className="flex items-center justify-center gap-1 whitespace-nowrap rounded-[var(--radius-sm)] border border-border bg-card px-1.5 py-2 text-[13px] font-semibold hover:border-selected">
            <span className="text-success">↓</span> Deposit
          </button>
          <button type="button" onClick={() => go('withdraw')} className="flex items-center justify-center gap-1 whitespace-nowrap rounded-[var(--radius-sm)] border border-border bg-card px-1.5 py-2 text-[13px] font-semibold hover:border-selected">
            <span className="text-danger">↑</span> Withdraw
          </button>
          <button type="button" onClick={() => go('close')} className="flex items-center justify-center gap-1 whitespace-nowrap rounded-[var(--radius-sm)] border border-border bg-card px-1.5 py-2 text-[13px] font-semibold hover:border-selected">
            <LockIcon size={13} /> Close
          </button>
        </div>
        <button type="button" onClick={toggleHidden} className="mt-2 flex w-full items-center gap-2 rounded-[var(--radius-sm)] px-1 py-1.5 text-[13px] text-muted-foreground hover:text-foreground">
          {hidden ? <Eye size={15} /> : <EyeOff size={15} />} {hidden ? 'Show the amount on the bar' : 'Hide the amount on the bar'}
        </button>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
