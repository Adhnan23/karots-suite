import { createContext, useContext } from 'react'
import type { TillCopy } from '@/api/types.gen'
import { lotsFor } from '@/features/till/cart/lines'
import type { Action } from '@/features/till/cart/store'
import type { TillState } from '@/features/till/cart/types'
import type { TillApi } from '@/plugins/slots'

const missing = (name = 'That product') => `${name} isn't on this phone yet`

/** The plugins' till, on the phone: the PC till's API over the same reducer, with
 * products from the copy. */
export function phoneTillApi(cart: TillState, dispatch: (a: Action) => void, copy: TillCopy): TillApi {
  const product = (id: number) => copy.products.find((p) => p.id === id)
  return {
    addServiceLine: (line) => dispatch({ type: 'addService', line }),
    addServiceGroup: (group) => dispatch({ type: 'addServiceGroup', group }),
    addPayout: (line) => dispatch({ type: 'addPayout', line }),
    addGroup: async (g) => {
      const items = []
      for (const it of g.items) {
        const p = product(it.productId)
        if (!p) return `${missing('An item')}: the group wasn't added`
        items.push({ product: p, per: it.per, discount: it.discount })
      }
      dispatch({ type: 'addGroup', group: { label: g.label, count: g.count, items, take: g.take, plugin: g.plugin } })
      return null
    },
    swapLine: async (key, productId, qty) => {
      const p = product(productId)
      if (!p) return missing()
      dispatch({ type: 'swap', key, product: p, qty })
      return null
    },
    saleType: cart.saleType,
    lines: cart.lines,
    refreshDrawer: () => {}, // the drawer comes from the copy, which refreshes itself
    customerId: cart.customerId,
    setCustomer: (id) => dispatch({ type: 'customer', id }),
    addProduct: async (productId, opts) => {
      const p = product(productId)
      if (!p) return missing()
      dispatch({ type: 'add', product: p, lots: [], units: opts?.units, plugin: opts?.plugin, discount: opts?.discount })
      return null
    },
    addTender: (method, amount, meta) => dispatch({ type: 'addTender', payment: { method, amount, reference: '', accountId: '', meta: meta ?? {} } }),
    setLineQty: (key, qty) => dispatch({ type: 'setQty', key, qty, lots: lotsFor(copy.priceOptions, cart.lines.find((l) => l.key === key)?.productId ?? 0) }),
    saleMeta: cart.meta ?? {},
    setSaleMeta: (key, value) => dispatch({ type: 'saleMeta', key, value }),
  }
}

const TillContext = createContext<TillApi | null>(null)
export const PhoneTillProvider = TillContext.Provider
/** Null outside the signed-in phone, so a sheet can be shown on its own. */
export const usePhoneTill = () => useContext(TillContext)
