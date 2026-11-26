import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { FolderOpen, Zap } from 'lucide-react'
import { useEffect } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import { slots } from '@/plugins/slots'
import type { PaletteEntry } from '@/theme/nav'
import { pageActions } from '@/theme/pageActions'
import { configQuery, errMsg, groupsQuery, heldQuery } from '../api'
import { useAskHold } from '../cart-ui/useHold'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'
import { useCheckoutCtx } from './CheckoutProvider'

/** Every button on the Sell page, in Ctrl K: type a few letters, Enter. The keys
 * cover the everyday ones; this reaches the rest without a mouse. */
export function SellActions() {
  const cfg = useSuspenseQuery(configQuery).data
  const ui = useTillUi()
  const co = useCheckoutCtx()
  const askHold = useAskHold()
  const { state, dispatch } = useTill()
  const held = useQuery(heldQuery).data ?? []
  const groups = useQuery(groupsQuery).data ?? []

  useEffect(() => {
    const a = (key: string, label: string, run: () => void): PaletteEntry => ({ key: `sell:${key}`, label, run, icon: Zap })
    const money = !cfg.sendsToCashier
    pageActions.current = [
      a('new', 'New sale (F12)', () => (state.lines.length ? ui.open({ kind: 'clearSale' }) : co.newSale())),
      a('hold', 'Hold the sale (F4)', () => askHold()),
      a('held', `Held sales (F5) · ${held.length}`, () => ui.open({ kind: 'held' })),
      a('customer', 'Customer (F8)', () => document.getElementById('till-customer')?.click()),
      a('addCustomer', 'Add a customer', () => ui.open({ kind: 'addCustomer' })),
      a('quick', 'Quick item: sell something not in the catalogue', () => ui.open({ kind: 'quickItem', barcode: '', name: '' })),
      a('retail', 'Retail prices', () => dispatch({ type: 'saleType', saleType: 'retail' })),
      a('wholesale', 'Wholesale prices', () => dispatch({ type: 'saleType', saleType: 'wholesale' })),
      ...(money
        ? [
            a('deposit', 'Deposit into the drawer', () => ui.open({ kind: 'deposit' })),
            a('withdraw', 'Withdraw from the drawer', () => ui.open({ kind: 'withdraw' })),
            ...(cfg.openCashDrawer
              ? [a('drawer', 'Open the cash drawer', () => void api('/till/drawer/no-sale', { method: 'POST' }).then(() => toast.success('Drawer opened')).catch((e) => toast.error(errMsg(e))))]
              : []),
            a('close', 'Close the till', () => ui.open({ kind: 'close' })),
          ]
        : []),
      ...slots.tillActions.map((x) => a(`action:${x.key}`, x.label, () => ui.open({ kind: 'action', key: x.key }))),
      ...slots.tillMenu.map((m) => a(`menu:${m.key}`, `${m.emoji} ${m.label}`, () => { ui.setSearch(''); ui.setStack([]); ui.setMenu(m.key) })),
      { key: 'sell:frequent', label: 'Frequently sold', run: ui.reset, icon: FolderOpen },
      ...groups.map((g) => ({ key: `sell:group:${g.id}`, label: `${g.emoji ?? ''} ${g.name}`.trim(), icon: FolderOpen, run: () => {
        ui.setSearch('')
        ui.setMenu(null)
        ui.setStack([g])
        document.getElementById('till-search')?.focus() // ↑ ↓ then pick from the group
      } })),
    ]
  })
  useEffect(() => () => { pageActions.current = [] }, [])
  return null
}
