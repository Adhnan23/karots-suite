import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { PresenceRow } from '@/api/types.gen'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { slots } from '@/plugins/slots'
import { adminNav, whereIs } from '@/theme/nav'
import { MORE, PRIMARY } from '@/theme/tillTabList'

/** Who's on now (admin only), every 5 s while a screen shows it. */
export const presenceQuery = queryOptions({
  queryKey: ['admin', 'presence'],
  queryFn: () => api<PresenceRow[]>('/admin/presence'),
  refetchInterval: 5_000,
})

const time = (iso: string) => new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })

/** A heartbeat's path as a place in the app ("Till · Returns", "Stock", "Lock screen"). */
export function placeOf(path: string): string {
  if (path === '/lock') return 'Lock screen'
  if (path === '/admin' || path === '/admin/') return 'Home'
  if (path.startsWith('/account')) return 'Changing their PIN'
  if (path.startsWith('/print')) return 'Printing'
  if (path === '/till' || path.startsWith('/till/')) {
    const tabs = [...PRIMARY, ...MORE, ...slots.tillTabs.map((t) => ({ to: `/till/x/${t.key}`, label: t.label }))]
    const tab = tabs.filter((t) => path === t.to || path.startsWith(t.to + '/')).sort((a, b) => b.to.length - a.to.length)[0]
    return `Till · ${tab?.label ?? 'Sell'}`
  }
  const page = whereIs(adminNav, path)?.page.label ?? slots.apps.find((a) => path === a.to || path.startsWith(a.to + '/'))?.label
  return page ?? path
}

const DOT: Record<string, string> = {
  active: 'bg-success shadow-[0_0_0_4px_color-mix(in_srgb,var(--success)_20%,transparent)]',
  idle: 'bg-warning',
  locked: 'bg-brand',
  away: 'bg-faint',
  signedOut: 'border-2 border-faint bg-transparent',
}

export function StateDot({ state }: { state: string }) {
  return <span aria-hidden className={cn('inline-block size-2.5 flex-none rounded-full', DOT[state] ?? DOT.away)} />
}

/** "Active", "Idle 5 min", "Locked", "Away since 10:42", "Signed out 10:42". */
export function stateText(r: PresenceRow): string {
  switch (r.state) {
    case 'active': return 'Active'
    case 'idle': return `Idle ${Math.floor(r.idleSec / 60)} min`
    case 'locked': return 'Locked'
    case 'away': return `Away since ${time(r.lastSeen)}`
    default: return `Signed out ${time(r.lastSeen)}`
  }
}

const items = (n: number) => `${Number.isInteger(n) ? n : n.toFixed(2)} item${n === 1 ? '' : 's'}`

/** One device: who, how they are, where — then the till, the cart and the machine. */
export function PresenceItem({ r, sym }: { r: PresenceRow; sym: string }) {
  const on = r.state !== 'away' && r.state !== 'signedOut'
  const cart = r.cartItems > 0 && on && (r.state === 'locked' ? `${items(r.cartItems)} waiting in the cart` : `Serving · ${items(r.cartItems)} · ${money(sym, r.cartTotal || '0')}`)
  const where = r.ip === '127.0.0.1' || r.ip === '::1' ? 'this PC (the server)' : r.ip
  const facts = [
    r.tillOpened && `Till open since ${time(r.tillOpened)}`,
    r.salesToday > 0 && `${r.salesToday} sale${r.salesToday === 1 ? '' : 's'} today`,
    [r.label, where].filter(Boolean).join(' · '),
  ].filter(Boolean)
  return (
    <li className={cn('flex items-start gap-3 py-2.5', !on && 'opacity-60')}>
      <span className="grid size-9 flex-none place-items-center rounded-full bg-brand-soft font-display font-bold">{r.name.slice(0, 1).toUpperCase()}</span>
      <span className="min-w-0 flex-1 text-sm">
        <span className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
          <b className="truncate">{r.name}</b>
          <span className="text-[11px] font-bold text-faint uppercase">{r.role}</span>
        </span>
        <span className="flex items-center gap-2"><StateDot state={r.state} /><span>{stateText(r)}{on && <> · <b>{placeOf(r.path)}</b></>}</span></span>
        {cart && <span className="block font-semibold">{cart}</span>}
        <small className="block text-faint [overflow-wrap:anywhere]">{facts.join(' · ')}</small>
      </span>
    </li>
  )
}
