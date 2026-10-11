import { Home as HomeIcon, Store, ArrowLeftRight, Banknote, BarChart3, Barcode, CalendarClock, ChefHat, ClipboardCheck, Coins, Database, FolderTree, HandCoins, History, LayoutDashboard, LayoutGrid, Puzzle, ListChecks, Package, PackageCheck, ReceiptText, RefreshCw, Ruler, ScanBarcode, Settings, ShieldCheck, ShoppingBag, ShoppingCart, Shuffle, TrendingUp, Truck, Undo2, UserCog, Users, Vault, Wallet, Warehouse, Wrench } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { Me } from '@/api/types.gen'
import { REPORTS } from '@/features/reports/hub'
import { slots, type AppEntry } from '@/plugins/slots'

export type { AppEntry }

/** gate 'stockTake': shown only while Settings → stock-take & intake is on (karots-pos toggle). */
/** gate 'admin': admins only (Users). gate 'tax': only for a tax-registered shop (Tax report). */
export type NavPage = { to: string; icon: LucideIcon; label: string; description: string; badge?: number; gate?: 'stockTake' | 'admin' | 'tax' }
/** noTabs: too many pages for the tab row (Reports) — the shelf and Ctrl K list them. */
/** blurb: the shelf's lead card line while no page is pointed at. */
export type NavArea = { key: string; label: string; icon: LucideIcon; blurb?: string; pages: NavPage[]; noTabs?: boolean }

/** karots-pos's admin areas and link descriptions (templates/layouts/admin.templ).
 * Each area's `pages` fills in as its milestone ports it; an area with none stays
 * off the dock. */
export const adminNav: NavArea[] = [
  {
    key: 'sell', label: 'Sell', icon: ShoppingCart, blurb: 'What was sold, who bought it, and what they owe.', pages: [
      { to: '/admin/sales', icon: ReceiptText, label: 'Sales', description: 'Every sale — reprint, export, take goods back' },
      { to: '/admin/customers', icon: Users, label: 'Customers', description: 'Accounts, credit limits & statements' },
      { to: '/admin/warranty', icon: ShieldCheck, label: 'Warranty', description: 'Units sold with cover — swap a faulty one, claim by receipt' },
    ],
  },
  {
    key: 'stock', label: 'Inventory', icon: Package, blurb: 'What’s on the shelf, what’s running out, what’s going off.', pages: [
      { to: '/admin/products', icon: Package, label: 'Products', description: 'Product catalogue & pricing' },
      { to: '/admin/products/review', icon: ListChecks, label: 'Items to review', description: 'Quick-added items needing details' },
      { to: '/admin/stock', icon: Warehouse, label: 'Stock', description: 'On-hand levels and movement history' },
      { to: '/admin/stock/batches', icon: CalendarClock, label: 'Batches', description: 'Live lots, expiry alerts and write-offs' },
      { to: '/admin/stock/take', icon: ClipboardCheck, label: 'Stock-take', description: 'Count and reconcile on-hand quantities', gate: 'stockTake' },
      { to: '/admin/inventory/intake', icon: ScanBarcode, label: 'Stock intake', description: 'Add or restock items, set quantity, and print labels in one place', gate: 'stockTake' },
      { to: '/admin/conversions', icon: Shuffle, label: 'Conversions', description: 'Repack / break bulk into sellable units' },
      { to: '/admin/services', icon: ChefHat, label: 'Services & recipes', description: 'Non-stocked lines and what each one consumes' },
    ],
  },
  {
    key: 'buy', label: 'Purchasing', icon: ShoppingBag, blurb: 'What’s coming in, what to order, and what you owe suppliers.', pages: [
      { to: '/admin/purchases', icon: PackageCheck, label: 'Purchases', description: 'Record goods received (GRN)' },
      { to: '/admin/reorder', icon: RefreshCw, label: 'Reorder', description: 'Items at or below reorder level — order them in one go' },
      { to: '/admin/purchase-returns', icon: Undo2, label: 'Purchase returns', description: 'Return stock to suppliers' },
      { to: '/admin/suppliers', icon: Truck, label: 'Suppliers', description: 'Supplier accounts & payables' },
    ],
  },
  {
    key: 'money', label: 'Money', icon: Wallet, blurb: 'Where the money is, how it moved, and what it earned.', pages: [
      { to: '/admin/finance', icon: TrendingUp, label: 'Finance', description: 'Revenue, profit, and the money owed either way' },
      { to: '/admin/cashflow', icon: ArrowLeftRight, label: 'Cash flow', description: 'Where the money is, how it moved, and what the business is worth' },
      { to: '/admin/lockers', icon: Vault, label: 'Lockers', description: 'Safes, bank accounts and pockets — balances and ledgers' },
      { to: '/admin/tills', icon: Banknote, label: 'Till sessions', description: 'Every open and close — expected, counted, over or short' },
      { to: '/admin/receipts', icon: ReceiptText, label: 'Receipts', description: 'Sale, cash, credit and warranty slips — find and reprint' },
      { to: '/admin/expenses', icon: HandCoins, label: 'Expenses', description: 'Rent, bills and other costs — and where the money came from' },
    ],
  },
  {
    key: 'reports', label: 'Reports', icon: BarChart3, noTabs: true, blurb: 'Every report — pick a range, then print or export.',
    // A getter: plugins register their report cards after this module loads.
    get pages(): NavPage[] {
      return [
        { to: '/admin/reports', icon: LayoutGrid, label: 'All reports', description: 'Sales, money, stock and suppliers — set a range, export or print' },
        ...REPORTS.filter((r) => r.to.startsWith('/admin/reports/')).map((r): NavPage => ({ to: r.to, icon: r.icon, label: r.title, description: r.desc, gate: r.gate })),
        ...slots.reportCards.map((c): NavPage => ({ to: c.href, icon: Puzzle, label: c.title, description: c.desc })),
      ]
    },
  },
  {
    key: 'setup', label: 'Setup', icon: Wrench, blurb: 'How the shop, the till and the staff are set up.', pages: [
      { to: '/admin/categories', icon: FolderTree, label: 'Categories', description: 'Product categories' },
      { to: '/admin/groups', icon: LayoutDashboard, label: 'Cashier menu', description: 'Group cards shown on the till' },
      { to: '/admin/units', icon: Ruler, label: 'Units', description: 'Units of measure' },
      { to: '/admin/denominations', icon: Coins, label: 'Denominations', description: 'Notes and coins for the drawer count' },
      { to: '/admin/labels', icon: Barcode, label: 'Barcode labels', description: 'Print product labels' },
      { to: '/admin/users', icon: UserCog, label: 'Users', description: 'Staff accounts & roles', gate: 'admin' },
      { to: '/admin/settings', icon: Settings, label: 'Settings', description: 'Shop name, receipt, tax, till cash rules, printers' },
      { to: '/admin/data', icon: Database, label: 'Data', description: 'Backups, restore and spreadsheets in and out', gate: 'admin' },
      { to: '/admin/activity', icon: History, label: 'Activity', description: 'Who did what — changes, stock, the drawer and lockers', gate: 'admin' },
    ],
  },
]

/** The nav this user sees: gated pages dropped, empty areas dropped, live
 * badges (page path → count) attached. */
export function navFor(me: Me, badges: Record<string, number> = {}): NavArea[] {
  return adminNav
    .map((a) => ({ ...a, pages: a.pages.filter((p) => (p.gate !== 'stockTake' || me.stockTakeEnabled) && (p.gate !== 'admin' || me.user.role === 'admin') && (p.gate !== 'tax' || me.taxRegistered)).map((p) => (badges[p.to] ? { ...p, badge: badges[p.to] } : p)) }))
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

/** The dock's Apps tile (theme.md §6): whatever the compiled-in plugins registered. */
export const apps: AppEntry[] = slots.apps

export type PaletteEntry = { key: string; label: string; to: string; icon: LucideIcon }

/** Ctrl K's default list: Home, every page the user can reach, the plugin apps, the till. */
export function paletteItems(areas: NavArea[]): PaletteEntry[] {
  return [
    { key: 'home', label: 'Home', to: '/admin', icon: HomeIcon },
    ...areas.flatMap((a) => a.pages.map((p) => ({ key: `${a.key}:${p.to}`, label: p.label, to: p.to, icon: a.icon }))),
    ...apps.map((a) => ({ key: `app:${a.key}`, label: a.label, to: a.to, icon: a.icon })),
    { key: 'till', label: 'Open the till', to: '/till', icon: Store },
  ]
}
