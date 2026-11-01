import { Menu as MenuIcon, PauseCircle, ShoppingCart } from 'lucide-react'
import { useCallback, useEffect, useReducer, useRef, useState } from 'react'
import { api } from '@/api/client'
import type { TillCopy } from '@/api/types.gen'
import { MOVED_LABEL, toHeldBlob } from '@/features/till/cart/held'
import { loadCart, saveCart } from '@/features/till/cart/persist'
import { initialState, tillReducer } from '@/features/till/cart/store'
import { saleTotal } from '@/features/till/cart/totals'
import { dec, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { setReloadGuard } from '@/lib/version'
import { GLASS, Screen } from '../bits'
import type { Who } from '../session'
import type { PhoneId } from '../store/phone'
import { drawerOpen as isOpen } from '../drawer'
import { useIdle } from '../idle'
import { pending, type Action } from '../store/outbox'
import { movedTo, refresh, sendNow, type SyncState } from '../sync'
import { CartBar } from './CartBar'
import { Held } from './Held'
import { Menu } from './Menu'
import { OpenFirst } from './OpenFirst'
import { Pay } from './Pay'
import { Sell } from './Sell'
import { OfflineBanner, TopBar } from './TopBar'

type Tab = 'sell' | 'held' | 'menu'
const TABS: [Tab, string, typeof ShoppingCart][] = [['sell', 'Sell', ShoppingCart], ['held', 'Held', PauseCircle], ['menu', 'Menu', MenuIcon]]

/** The signed-in phone: top bar, Sell / Held / Menu, and the amber state while
 * the PC is down. The cart is the desktop till's own (same reducer, same pricing). */
export function Phone({ who, copy, sync, onSignOut }: { who: Who; copy: TillCopy | null; sync: SyncState; phone: PhoneId; onSignOut: (why?: string) => void }) {
  const [tab, setTab] = useState<Tab>('sell')
  const [paying, setPaying] = useState(false)
  const [cart, dispatch] = useReducer(tillReducer, who.id, (id) => loadCart(id) ?? initialState(copy?.defaultSaleType === 'wholesale' ? 'wholesale' : 'retail'))
  useEffect(() => saveCart(who.id, cart), [who.id, cart])
  // What waits to be sent: the drawer and Held count it until the copy does.
  const [out, setOut] = useState<Action[]>([])
  const [bump, setBump] = useState(0)
  useEffect(() => { void pending().then(setOut, () => {}) }, [sync.waiting, sync.copyAt, bump])
  const sent = useCallback(() => { setBump((n) => n + 1); void refresh() }, [])
  const drawerOpen = !!copy && isOpen(copy, who.id, out)
  // One person, one device: signed in on the PC since, they moved there. Park the
  // cart in Held (as the PC does when they come to the phone) and lock.
  const latest = useRef({ cart, onSignOut })
  const payingRef = useRef(paying)
  useEffect(() => { payingRef.current = paying })
  useEffect(() => { latest.current = { cart, onSignOut } })
  useEffect(() => {
    if (!sync.live) return
    const id = setInterval(() => void movedTo(who.id).then(async (to) => {
      const { cart, onSignOut } = latest.current
      if (!to) return
      if (cart.lines.length) {
        await sendNow({ kind: 'hold', userId: who.id, body: { label: MOVED_LABEL, saleType: cart.saleType, customerId: cart.customerId ?? undefined,
          discount: String(num(cart.billDiscount)), cart: toHeldBlob(cart), itemCount: cart.lines.length, total: dec(saleTotal(cart)) } }, () => null)
        dispatch({ type: 'newSale', saleType: cart.saleType })
      }
      onSignOut(`${who.name} is now on ${to}. Sign in here to take it back.`)
    }), 15_000)
    return () => clearInterval(id)
  }, [sync.live, who])
  const switchPerson = () => { if (sync.live) void api('/auth/lock', { method: 'POST' }).catch(() => {}); onSignOut() }
  // A new version reloads the app only with no sale going and no sheet open (a
  // float half counted, a quick item half typed) — as the PC till.
  useEffect(() => setReloadGuard(() => !latest.current.cart.lines.length && !payingRef.current && !document.querySelector('[role="dialog"]')), [])
  useIdle(copy?.lockTimeoutMinutes ?? 0, () => {
    if (sync.live) void api('/auth/lock', { method: 'POST' }).catch(() => {})
    onSignOut(`Locked after ${copy?.lockTimeoutMinutes} min with no use. Sign in to carry on.`)
  })
  if (!copy) return <Screen><p className="p-6 text-center text-muted-foreground">Getting the shop’s copy…</p></Screen>

  return (
    <Screen fixed>
      {!(tab === 'sell' && paying) && <TopBar who={who} sync={sync} drawerOpen={drawerOpen} onSwitch={switchPerson} />}
      {!sync.live && <OfflineBanner copyAt={sync.copyAt} />}
      <main className="flex min-h-0 flex-1 flex-col">
        {tab === 'sell' && !drawerOpen && <OpenFirst who={who} copy={copy} onSent={sent} />}
        {tab === 'sell' && drawerOpen && paying && <Pay who={who} copy={copy} cart={cart} dispatch={dispatch} live={sync.live} onBack={() => { setPaying(false); sent() }} />}
        {tab === 'sell' && drawerOpen && !paying && (
          <>
            <Sell copy={copy} cart={cart} dispatch={dispatch} live={sync.live} />
            <CartBar cart={cart} dispatch={dispatch} copy={copy} onPay={() => setPaying(true)} />
          </>
        )}
        {tab === 'held' && <Held who={who} copy={copy} out={out} cart={cart} dispatch={dispatch} onSent={sent} onResumed={() => { setPaying(false); setTab('sell') }} />}
        {tab === 'menu' && <Menu who={who} copy={copy} out={out} live={sync.live} onSent={sent} onSwitch={switchPerson} />}
      </main>
      <nav aria-label="Phone tabs" className={cn('flex justify-around border-t border-border bg-bar px-2 pt-1.5 pb-3 text-[10.5px] font-semibold text-faint', GLASS)}>
        {TABS.map(([k, label, Icon]) => (
          <button key={k} type="button" aria-current={tab === k ? 'page' : undefined} onClick={() => setTab(k)}
            className="flex min-w-16 flex-col items-center gap-0.5 aria-[current=page]:text-foreground"><Icon size={19} />{label}</button>
        ))}
      </nav>
    </Screen>
  )
}
