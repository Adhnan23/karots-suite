import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { DrawerState, SaleResult, TillConfig } from '@/api/types.gen'
import { configQuery, customersQuery, drawerQuery } from '../api'
import { TillProvider, useTill } from '../TillProvider'
import { useCheckout } from './useCheckout'

const cfg: TillConfig = { symbol: 'Rs.', defaultSaleType: 'retail', askToPrint: false, openCashDrawer: false, allowUntrackedCash: true,
  defaultLockerId: 0, allowContinueLast: true, canManageCredit: false, canHandleSuppliers: false, canSeeCost: false, canChangePin: true, isManager: false, tenderMethods: [], labelSize: '50x25', shop: { name: 'Shop', address: '', phone: '', footer: '' }, receiptWidth: '80' }
const drawer: DrawerState = { summary: { cashSales: '0', payIns: '0', withdrawals: '0', expected: '0' }, denominations: [], lockers: [] }
const sale: SaleResult = { id: 42, receiptNo: 'S-1', total: '100.00', paidAmount: '150.00', changeGiven: '50.00', roundingToAccount: '0.00', customerName: '', methods: ['cash'] }

function setup() {
  const qc = new QueryClient()
  qc.setQueryData(configQuery.queryKey, cfg)
  qc.setQueryData(drawerQuery.queryKey, drawer)
  qc.setQueryData(customersQuery.queryKey, [])
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}><TillProvider userId={1} saleType="retail">{children}</TillProvider></QueryClientProvider>
  )
  return renderHook(() => ({ till: useTill(), co: useCheckout() }), { wrapper })
}

describe('useCheckout', () => {
  let calls: string[]
  beforeEach(() => {
    sessionStorage.clear()
    calls = []
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      calls.push(url)
      await new Promise((r) => setTimeout(r, 20))
      return new Response(url.endsWith('/till/sales') ? JSON.stringify(sale) : null, { status: url.endsWith('/till/sales') ? 201 : 204 })
    }))
  })
  afterEach(() => vi.unstubAllGlobals())

  it('a double Enter / double click posts exactly one sale', async () => {
    const { result } = setup()
    act(() => result.current.till.dispatch({ type: 'addService', line: { productId: 1, name: 'x', price: 100 } }))
    await act(async () => {
      await Promise.all([result.current.co.run(), result.current.co.run()])
    })
    expect(calls.filter((u) => u.endsWith('/till/sales'))).toHaveLength(1)
  })

  it('auto-print mode: prints with the drawer kick merged, clears the cart, shows the change', async () => {
    const { result } = setup()
    act(() => result.current.till.dispatch({ type: 'addService', line: { productId: 1, name: 'x', price: 100 } }))
    await act(async () => { await result.current.co.run() })
    expect(calls.some((u) => u.endsWith('/till/sales/42/print?kick=1'))).toBe(true)
    expect(result.current.till.state.lines).toEqual([])
    expect(result.current.till.state.postSaleChange).toBe(50)
  })
})
