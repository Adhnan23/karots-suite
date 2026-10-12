import { useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { createContext, useContext, useMemo, useState, type ReactNode } from 'react'
import { api } from '@/api/client'
import type { Lot, TillGroup, TillProduct } from '@/api/types.gen'
import type { TillApi } from '@/plugins/slots'
import { configQuery, drawerQuery, errMsg } from './api'
import { printMoneySlip, slipPath } from './print'
import { askToPrint } from './SlipAsk'
import { useTill } from './TillProvider'

/** One till dialog at a time (karots-pos anyModalOpen). */
export type Dialog =
  | { kind: 'withdraw' }
  | { kind: 'deposit' }
  | { kind: 'close'; logout?: boolean }
  | { kind: 'held' }
  | { kind: 'clearSale' }
  | { kind: 'quickItem'; barcode: string; name: string }
  | { kind: 'addCustomer' }
  | { kind: 'info'; product: TillProduct }
  | { kind: 'pricePick'; product: TillProduct; lots: Lot[] }
  | { kind: 'suggestion'; product: TillProduct }
  | { kind: 'action'; key: string; text?: string }

type Ui = {
  dialog: Dialog | null
  open: (d: Dialog) => void
  close: () => void
  search: string
  setSearch: (s: string) => void
  stack: TillGroup[] // group drill-down breadcrumb; [] = top menu
  setStack: (s: TillGroup[]) => void
  menu: string | null // open plugin menu root (slots.tillMenu key)
  menuProduct: TillProduct | null // the picked product a menu root claimed
  setMenu: (k: string | null, product?: TillProduct) => void
  /** F1 / ⟲ Menu: back to the top menu and refocus the scanner. NEVER touches the cart. */
  reset: () => void
  /** After a till cash move: prompt to print the CR- slip, or auto-print it. */
  afterMove: (receiptId: number | undefined, mergeKick?: boolean) => void
}

const UiCtx = createContext<Ui | null>(null)

export function TillUiProvider({ children }: { children: ReactNode }) {
  const cfg = useSuspenseQuery(configQuery).data
  const [dialog, setDialog] = useState<Dialog | null>(null)
  const [search, setSearch] = useState('')
  const [stack, setStack] = useState<TillGroup[]>([])
  const [menu, setMenuKey] = useState<string | null>(null)
  const [menuProduct, setMenuProduct] = useState<TillProduct | null>(null)
  const value = useMemo<Ui>(() => ({
    dialog, open: setDialog, close: () => setDialog(null),
    search, setSearch, stack, setStack, menu, menuProduct,
    setMenu: (k, product) => {
      setMenuKey(k)
      setMenuProduct(product ?? null)
    },
    reset: () => {
      setSearch('')
      setStack([])
      setMenuKey(null)
      setMenuProduct(null)
      document.getElementById('till-search')?.focus()
    },
    afterMove: (id, mergeKick = false) => {
      if (!id) return
      if (cfg.askToPrint) askToPrint({ path: slipPath.money(id), title: 'Print the receipt?', note: 'A cash receipt was recorded for this drawer move.' })
      else void printMoneySlip(id, mergeKick)
    },
  }), [dialog, search, stack, menu, menuProduct, cfg.askToPrint])
  return <UiCtx.Provider value={value}>{children}</UiCtx.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export function useTillUi() {
  const v = useContext(UiCtx)
  if (!v) throw new Error('useTillUi must be used inside <TillUiProvider>')
  return v
}

/** The narrow surface plugin till components get (slots.ts). */
// eslint-disable-next-line react-refresh/only-export-components
export function useTillApi(): TillApi {
  const { state, dispatch } = useTill()
  const qc = useQueryClient()
  const saleType = state.saleType
  return useMemo(() => ({
    addServiceLine: (line) => dispatch({ type: 'addService', line }),
    addGroup: async (g) => {
      const items = []
      for (const it of g.items) {
        try {
          const product = await qc.fetchQuery({ queryKey: ['till', 'product', it.productId], queryFn: () => api<TillProduct>(`/till/products/${it.productId}`), staleTime: 30_000 })
          items.push({ product, per: it.per, discount: it.discount })
        } catch (e) {
          return `${errMsg(e, 'An item')} — the group wasn't added`
        }
      }
      dispatch({ type: 'addGroup', group: { label: g.label, count: g.count, items, take: g.take, plugin: g.plugin } })
      return null
    },
    swapLine: async (key, productId) => {
      try {
        const product = await qc.fetchQuery({ queryKey: ['till', 'product', productId], queryFn: () => api<TillProduct>(`/till/products/${productId}`), staleTime: 30_000 })
        dispatch({ type: 'swap', key, product })
        return null
      } catch (e) {
        return errMsg(e, "Couldn't swap")
      }
    },
    saleType,
    refreshDrawer: () => void qc.invalidateQueries({ queryKey: drawerQuery.queryKey }),
  }), [dispatch, qc, saleType])
}
