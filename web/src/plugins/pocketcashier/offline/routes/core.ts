import type { DrawerState, ProductPage, TillConfig, TillCustomer } from '@/api/types.gen'
import { search } from '../../catalog'
import { drawerOpen, expectedCash } from '../../drawer'
import { withOverlay } from '../../overlay'
import { tempNumber, type OfflineRoute, type RouteCtx } from '../fetch'

const shown = (c: RouteCtx) => (c.copy ? withOverlay(c.copy, c.out) : null)

function config(c: RouteCtx): Partial<TillConfig> {
  const copy = c.copy
  const me = copy?.people.find((p) => p.id === c.userId)
  return {
    symbol: copy?.symbol ?? '', defaultSaleType: copy?.defaultSaleType ?? 'retail', askToPrint: copy?.askToPrint ?? false, openCashDrawer: false,
    allowUntrackedCash: copy?.allowUntrackedCash ?? false, defaultLockerId: copy?.defaultLockerId ?? 0, allowContinueLast: copy?.allowContinueLast ?? false,
    canManageCredit: me?.canManageCredit ?? false, canSeeCost: me?.canSeeCost ?? false, canHandleSuppliers: false, canSeeLowStock: false, canChangePin: false,
    sendsToCashier: false, isManager: me?.role === 'manager', tenderMethods: copy?.tenderMethods ?? [], shop: copy?.shop, labelStyles: {},
  }
}

function drawer(c: RouteCtx): Partial<DrawerState> | null {
  if (!c.copy) return null
  const session = c.copy.sessions.find((s) => s.userId === c.userId)
  const open = drawerOpen(c.copy, c.userId, c.out)
  return {
    summary: {
      session: open ? { id: session?.id ?? 0, openingCash: session?.openingCash ?? '0', openedAt: session?.openedAt ?? '' } : undefined,
      cashSales: '0', payIns: '0', withdrawals: '0', expected: expectedCash(c.copy, c.userId, c.out), stale: false,
    },
    denominations: c.copy.denominations ?? [],
    lockers: c.copy.places.filter((p) => p.cashierAccess).map((p) => ({ id: p.id, name: p.name, kind: p.kind, balance: '0' })),
  }
}

const customers = (c: RouteCtx): TillCustomer[] => (shown(c)?.customers ?? []).map((x) => ({ id: x.id, name: x.name, phone: x.phone, creditLimit: x.creditLimit, outstanding: x.balance }))

/** What plugin screens read from the till itself, answered from the copy. */
export const coreRoutes: OfflineRoute[] = [
  { method: 'GET', path: '/till/config', answer: config },
  { method: 'GET', path: '/till/drawer', answer: drawer },
  { method: 'GET', path: '/till/customers', answer: customers },
  { method: 'GET', path: '/till/price-options', answer: (c) => c.copy?.priceOptions ?? {} },
  { method: 'GET', path: '/till/products', answer: (c): ProductPage => {
    const items = search(shown(c), c.query.get('search') ?? '')
    return { items, total: items.length }
  } },
  { method: 'GET', path: '/till/products/:id', answer: (c) => shown(c)?.products.find((p) => String(p.id) === c.params.id) ?? null },
  { method: 'GET', path: '/till/products/:id/lots', answer: (c) => c.copy?.priceOptions?.[c.params.id] ?? [] },
  // A plugin's own money screen (a repair deposit) uses the counter's credit form.
  { method: 'POST', path: '/till/customers/:id/payment', queue: true,
    label: (c) => `Credit from ${c.copy?.customers.find((x) => String(x.id) === c.params.id)?.name ?? 'a customer'}`,
    call: (c) => {
      const b = c.body as { amount?: string; method?: string }
      const had = c.copy?.customers.find((x) => String(x.id) === c.params.id)?.balance
      return { cashIn: b.method === 'cash' ? b.amount : undefined, saw: had === undefined ? undefined : { customerId: Number(c.params.id), balance: shown(c)?.customers.find((x) => String(x.id) === c.params.id)?.balance ?? had } }
    },
    answer: (c) => ({ paymentId: tempNumber(c.ref), receiptNo: '', balanceAfter: '' }) },
]
