import { useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { createContext, useContext, useMemo, useState, type ReactNode } from 'react'
import type { Lot, TillGroup, TillProduct } from '@/api/types.gen'
import type { TillApi } from '@/plugins/slots'
import { configQuery, drawerQuery } from './api'
import { printMoneySlip } from './print'
import { useTill } from './TillProvider'

/** One till dialog at a time (karots-pos anyModalOpen). */
export type Dialog =
  | { kind: 'withdraw' }
  | { kind: 'deposit' }
  | { kind: 'close'; logout?: boolean }
  | { kind: 'slip'; receiptId: number }
  | { kind: 'held' }
  | { kind: 'clearSale' }
  | { kind: 'quickItem'; barcode: string; name: string }
  | { kind: 'addCustomer' }
  | { kind: 'info'; product: TillProduct }
  | { kind: 'pricePick'; product: TillProduct; lots: Lot[] }
  | { kind: 'suggestion'; product: TillProduct }
  | { kind: 'action'; key: string }

type Ui = {
  dialog: Dialog | null
  open: (d: Dialog) => void
  close: () => void
  search: string
  setSearch: (s: string) => void
  stack: TillGroup[] // group drill-down breadcrumb; [] = top menu
  setStack: (s: TillGroup[]) => void
  menu: string | null // open plugin menu root (slots.tillMenu key)
  setMenu: (k: string | null) => void
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
  const [menu, setMenu] = useState<string | null>(null)
  const value = useMemo<Ui>(() => ({
    dialog, open: setDialog, close: () => setDialog(null),
    search, setSearch, stack, setStack, menu, setMenu,
    reset: () => {
      setSearch('')
      setStack([])
      setMenu(null)
      document.getElementById('till-search')?.focus()
    },
    afterMove: (id, mergeKick = false) => {
      if (!id) return
      if (cfg.askToPrint) setDialog({ kind: 'slip', receiptId: id })
      else void printMoneySlip(id, mergeKick)
    },
  }), [dialog, search, stack, menu, cfg.askToPrint])
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
  const { dispatch } = useTill()
  const qc = useQueryClient()
  return useMemo(() => ({
    addServiceLine: (line) => dispatch({ type: 'addService', line }),
    refreshDrawer: () => void qc.invalidateQueries({ queryKey: drawerQuery.queryKey }),
  }), [dispatch, qc])
}
