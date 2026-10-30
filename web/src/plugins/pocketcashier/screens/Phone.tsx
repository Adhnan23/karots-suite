import { Menu as MenuIcon, PauseCircle, ShoppingCart } from 'lucide-react'
import { useEffect, useReducer, useState } from 'react'
import { api } from '@/api/client'
import type { TillCopy } from '@/api/types.gen'
import { loadCart, saveCart } from '@/features/till/cart/persist'
import { initialState, tillReducer } from '@/features/till/cart/store'
import { cn } from '@/lib/utils'
import { GLASS, Screen } from '../bits'
import type { Who } from '../session'
import type { PhoneId } from '../store/phone'
import type { SyncState } from '../sync'
import { CartBar } from './CartBar'
import { Sell } from './Sell'
import { OfflineBanner, TopBar } from './TopBar'

type Tab = 'sell' | 'held' | 'menu'
const TABS: [Tab, string, typeof ShoppingCart][] = [['sell', 'Sell', ShoppingCart], ['held', 'Held', PauseCircle], ['menu', 'Menu', MenuIcon]]

/** The signed-in phone: top bar, Sell / Held / Menu, and the amber state while
 * the PC is down. The cart is the desktop till's own (same reducer, same pricing). */
export function Phone({ who, copy, sync, onSignOut }: { who: Who; copy: TillCopy | null; sync: SyncState; phone: PhoneId; onSignOut: () => void }) {
  const [tab, setTab] = useState<Tab>('sell')
  const [cart, dispatch] = useReducer(tillReducer, who.id, (id) => loadCart(id) ?? initialState(copy?.defaultSaleType === 'wholesale' ? 'wholesale' : 'retail'))
  useEffect(() => saveCart(who.id, cart), [who.id, cart])
  const drawerOpen = !!copy?.sessions.some((s) => s.userId === who.id)
  const switchPerson = () => { if (sync.live) void api('/auth/lock', { method: 'POST' }).catch(() => {}); onSignOut() }
  if (!copy) return <Screen><p className="p-6 text-center text-muted-foreground">Getting the shop’s copy…</p></Screen>

  return (
    <Screen>
      <TopBar who={who} sync={sync} drawerOpen={drawerOpen} onSwitch={switchPerson} />
      {!sync.live && <OfflineBanner copyAt={sync.copyAt} />}
      <main className="flex min-h-0 flex-1 flex-col">
        {tab === 'sell' && (
          <>
            <Sell copy={copy} cart={cart} dispatch={dispatch} live={sync.live} />
            <CartBar cart={cart} dispatch={dispatch} copy={copy} onPay={() => setTab('sell')} />
          </>
        )}
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
