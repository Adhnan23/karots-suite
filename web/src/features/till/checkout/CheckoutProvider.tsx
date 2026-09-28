import { createContext, useContext, type ReactNode } from 'react'
import { useCheckout } from './useCheckout'

type Co = ReturnType<typeof useCheckout>
const CoCtx = createContext<Co | null>(null)

export function CheckoutProvider({ children }: { children: ReactNode }) {
  return <CoCtx.Provider value={useCheckout()}>{children}</CoCtx.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export function useCheckoutCtx() {
  const v = useContext(CoCtx)
  if (!v) throw new Error('useCheckoutCtx must be used inside <CheckoutProvider>')
  return v
}
