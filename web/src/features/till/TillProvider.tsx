import { createContext, useContext, useEffect, useReducer, useRef, type Dispatch, type ReactNode } from 'react'
import { toast } from 'sonner'
import { setReloadGuard } from '@/lib/version'
import { loadCart, saveCart } from './cart/persist'
import { initialState, tillReducer, type Action } from './cart/store'
import type { SaleType, TillState } from './cart/types'

type Ctx = { state: TillState; dispatch: Dispatch<Action> }
const TillCtx = createContext<Ctx | null>(null)

/** The sale in progress. Lives at the till layout so M1b's till tabs keep it, and is
 * saved per user so auto-lock / reload bring it back. */
export function TillProvider({ userId, saleType, children }: { userId: number; saleType: SaleType; children: ReactNode }) {
  const [state, dispatch] = useReducer(tillReducer, undefined, () => loadCart(userId) ?? initialState(saleType))
  useEffect(() => saveCart(userId, state), [userId, state])
  // After an update the page reloads itself — never with a sale on the counter.
  const cartEmpty = useRef(true)
  useEffect(() => { cartEmpty.current = state.lines.length === 0 }, [state.lines.length])
  useEffect(() => setReloadGuard(() => cartEmpty.current), [])
  useEffect(() => {
    if (!state.notice) return
    ;(state.notice.tone === 'error' ? toast.error : toast.success)(state.notice.text)
    dispatch({ type: 'clearNotice' })
  }, [state.notice])
  return <TillCtx.Provider value={{ state, dispatch }}>{children}</TillCtx.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export function useTill() {
  const v = useContext(TillCtx)
  if (!v) throw new Error('useTill must be used inside <TillProvider>')
  return v
}
