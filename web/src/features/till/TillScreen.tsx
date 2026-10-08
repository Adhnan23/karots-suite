import { useQuery } from '@tanstack/react-query'
import { MotionConfig } from 'motion/react'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'
import { api } from '@/api/client'
import { cn } from '@/lib/utils'
import { drawerQuery } from './api'
import { CatalogPanel } from './catalog/CatalogPanel'
import { CartPanel } from './cart-ui/CartPanel'
import { CheckoutProvider } from './checkout/CheckoutProvider'
import { Hotkeys } from './checkout/Hotkeys'
import { Prompts } from './checkout/Prompts'
import { SaleComplete } from './checkout/SaleComplete'
import { OpenRegister } from './drawer/OpenRegister'
import { PayPanel } from './pay/PayPanel'
import { TillDialogs } from './TillDialogs'
import { TillUiProvider, useTillUi } from './TillUi'
import { PANEL } from './ui'

/** Sign-out was refused (TILL_OPEN): open the count in logout mode; if the till is
 * somehow already closed, just finish signing out. */
function LogoutClose({ logout }: { logout: boolean }) {
  const ui = useTillUi()
  const d = useQuery(drawerQuery).data
  const handled = useRef(false)
  useEffect(() => {
    if (!logout || !d || handled.current) return
    handled.current = true
    if (d.summary.session) ui.open({ kind: 'close', logout: true })
    else void api('/auth/logout', { method: 'POST' }).finally(() => window.location.assign('/login'))
  }, [logout, d, ui])
  return null
}

/** ?drawer=…: open the drawer move picked from the top bar's chip, then drop the
 * flag so a reload doesn't open it again. */
function DrawerMoveFromBar({ drawer }: { drawer?: 'deposit' | 'withdraw' | 'close' }) {
  const { open } = useTillUi()
  const navigate = useNavigate()
  const handled = useRef<string | undefined>(undefined)
  useEffect(() => {
    // Once per flag: opening the dialog re-renders the till, which must not reopen it.
    if (!drawer || handled.current === drawer) return
    handled.current = drawer
    open({ kind: drawer })
    void navigate({ to: '/till', search: {}, replace: true }).then(() => { handled.current = undefined })
  }, [drawer, open, navigate])
  return null
}

function Body() {
  const drawer = useQuery(drawerQuery).data
  return (
    <div className="relative grid h-full min-h-0 grid-cols-[1fr_360px] grid-rows-[minmax(0,2fr)_minmax(0,3fr)] gap-[14px] p-[14px] [grid-template-areas:'cat_cart'_'cat_pay'] min-[1200px]:grid-cols-[1fr_392px_356px] min-[1200px]:grid-rows-[minmax(0,1fr)] min-[1200px]:[grid-template-areas:'cat_cart_pay']">
      <section aria-label="Catalogue" className="flex min-h-0 flex-col gap-3 [grid-area:cat]">
        <CatalogPanel />
      </section>
      <section aria-label="Cart" className={cn(PANEL, 'flex min-h-0 flex-col [grid-area:cart]')}>
        <CartPanel />
      </section>
      <section aria-label="Payment" className={cn(PANEL, 'flex min-h-0 flex-col overflow-hidden [grid-area:pay]')}>
        <PayPanel />
      </section>
      {drawer && !drawer.summary.session && <OpenRegister state={drawer} />}
      <SaleComplete />
    </div>
  )
}

/** The sell screen (theme.md §7): catalogue | cart | pay. */
export function TillScreen({ logout = false, drawer }: { logout?: boolean; drawer?: 'deposit' | 'withdraw' | 'close' }) {
  return (
    <MotionConfig reducedMotion="user">
      <TillUiProvider>
        <CheckoutProvider>
          <Body />
          <TillDialogs />
          <Prompts />
          <Hotkeys />
          <LogoutClose logout={logout} />
          <DrawerMoveFromBar drawer={drawer} />
        </CheckoutProvider>
      </TillUiProvider>
    </MotionConfig>
  )
}
