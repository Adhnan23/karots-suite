import { BarChart3, Package, ShoppingBag, ShoppingCart, Wallet, Wrench } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { Me } from '@/api/types.gen'

/** gate 'stockTake': shown only while Settings → stock-take & intake is on (karots-pos toggle). */
/** gate 'admin': admins only (Users). */
export type NavPage = { to: string; label: string; description: string; badge?: number; gate?: 'stockTake' | 'admin' }
export type NavArea = { key: string; label: string; icon: LucideIcon; pages: NavPage[] }
export type AppEntry = { key: string; label: string; description: string; icon: LucideIcon; color: string }

/** karots-pos's admin areas and link descriptions (templates/layouts/admin.templ).
 * Each area's `pages` fills in as its milestone ports it; an area with none stays
 * off the dock. */
export const adminNav: NavArea[] = [
  {
    key: 'sell', label: 'Sell', icon: ShoppingCart, pages: [
      { to: '/admin/sales', label: 'Sales', description: 'Every sale — reprint, export, take goods back' },
      { to: '/admin/customers', label: 'Customers', description: 'Accounts, credit limits & statements' },
    ],
  },
  {
    key: 'stock', label: 'Inventory', icon: Package, pages: [
      { to: '/admin/products', label: 'Products', description: 'Product catalogue & pricing' },
      { to: '/admin/products/review', label: 'Items to review', description: 'Quick-added items needing details' },
      { to: '/admin/stock', label: 'Stock', description: 'On-hand levels and movement history' },
      { to: '/admin/stock/batches', label: 'Batches', description: 'Live lots, expiry alerts and write-offs' },
      { to: '/admin/stock/take', label: 'Stock-take', description: 'Count and reconcile on-hand quantities', gate: 'stockTake' },
      { to: '/admin/inventory/intake', label: 'Stock intake', description: 'Add or restock items, set quantity, and print labels in one place', gate: 'stockTake' },
      { to: '/admin/conversions', label: 'Conversions', description: 'Repack / break bulk into sellable units' },
      { to: '/admin/services', label: 'Services & recipes', description: 'Non-stocked lines and what each one consumes' },
    ],
  },
  {
    key: 'buy', label: 'Purchasing', icon: ShoppingBag, pages: [
      { to: '/admin/purchases', label: 'Purchases', description: 'Record goods received (GRN)' },
      { to: '/admin/reorder', label: 'Reorder', description: 'Items at or below reorder level — order them in one go' },
      { to: '/admin/purchase-returns', label: 'Purchase returns', description: 'Return stock to suppliers' },
      { to: '/admin/suppliers', label: 'Suppliers', description: 'Supplier accounts & payables' },
    ],
  },
  {
    key: 'money', label: 'Money', icon: Wallet, pages: [
      { to: '/admin/cashflow', label: 'Cash flow', description: 'Where the money is, how it moved, and what the business is worth' },
      { to: '/admin/lockers', label: 'Lockers', description: 'Safes, bank accounts and pockets — balances and ledgers' },
      { to: '/admin/tills', label: 'Till sessions', description: 'Every open and close — expected, counted, over or short' },
      { to: '/admin/receipts', label: 'Receipts', description: 'Sale, cash, credit and warranty slips — find and reprint' },
      { to: '/admin/expenses', label: 'Expenses', description: 'Rent, bills and other costs — and where the money came from' },
    ],
  },
  { key: 'reports', label: 'Reports', icon: BarChart3, pages: [] },
  {
    key: 'setup', label: 'Setup', icon: Wrench, pages: [
      { to: '/admin/categories', label: 'Categories', description: 'Product categories' },
      { to: '/admin/groups', label: 'Cashier menu', description: 'Group cards shown on the till' },
      { to: '/admin/units', label: 'Units', description: 'Units of measure' },
      { to: '/admin/denominations', label: 'Denominations', description: 'Notes and coins for the drawer count' },
      { to: '/admin/labels', label: 'Barcode labels', description: 'Print product labels' },
      { to: '/admin/users', label: 'Users', description: 'Staff accounts & roles', gate: 'admin' },
    ],
  },
]

/** The nav this user sees: gated pages dropped, empty areas dropped, live
 * badges (page path → count) attached. */
export function navFor(me: Me, badges: Record<string, number> = {}): NavArea[] {
  return adminNav
    .map((a) => ({ ...a, pages: a.pages.filter((p) => (p.gate !== 'stockTake' || me.stockTakeEnabled) && (p.gate !== 'admin' || me.user.role === 'admin')).map((p) => (badges[p.to] ? { ...p, badge: badges[p.to] } : p)) }))
    .filter((a) => a.pages.length > 0)
}

/** The area and page owning a path (longest matching page wins, so /admin/stock/take beats /admin/stock). */
export function whereIs(areas: NavArea[], path: string): { area: NavArea; page: NavPage } | undefined {
  let best: { area: NavArea; page: NavPage } | undefined
  for (const area of areas)
    for (const page of area.pages)
      if ((path === page.to || path.startsWith(page.to + '/')) && (!best || page.to.length > best.page.to.length)) best = { area, page }
  return best
}

/** Plugin slot for the dock's Apps tile (theme.md §6). Empty until M7. */
export const apps: AppEntry[] = []
