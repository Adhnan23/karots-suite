import { ArrowLeftRight, Boxes, CalendarClock, ClipboardList, Fuel, Calculator, FolderTree, Landmark, Receipt, HandCoins, Truck, Undo2, Users, PackageCheck, Clock, LineChart, PackageSearch, ReceiptText, Trophy, TrendingUp, UserRound, Wallet } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

export type ReportGroup = 'sales' | 'money' | 'stock'
/** gate 'tax': only for a tax-registered shop (karots-pos hides the Tax report otherwise). */
export type ReportEntry = { to: string; title: string; desc: string; group: ReportGroup; icon: LucideIcon; gate?: 'tax' }

/** karots-pos reportHubGroups: three groups, curated order (common first). Each
 * report adds itself here as it lands. */
export const GROUPS: [ReportGroup, string][] = [['sales', 'Sales and customers'], ['money', 'Money and profit'], ['stock', 'Inventory and suppliers']]

export const REPORTS: ReportEntry[] = [
  { to: '/admin/reports/sales', title: 'Sales', desc: 'Receipts, profit and the trend, against the period before', group: 'sales', icon: TrendingUp },
  { to: '/admin/reports/top-products', title: 'Top products', desc: 'Best sellers by revenue, units or profit', group: 'sales', icon: Trophy },
  { to: '/admin/reports/product-sales', title: 'Product sales', desc: 'One product over time, beside last year', group: 'sales', icon: LineChart },
  { to: '/admin/reports/sales-by-cashier', title: 'Sales by cashier', desc: 'Takings and discounts per cashier', group: 'sales', icon: UserRound },
  { to: '/admin/reports/peak-hours', title: 'Peak hours', desc: 'Busiest days and hours — plan staff and breaks', group: 'sales', icon: Clock },
  { to: '/admin/reports/returns', title: 'Returns', desc: 'Goods taken back and money refunded', group: 'sales', icon: Undo2 },
  { to: '/admin/reports/customer-dues', title: 'Customer dues', desc: 'Who owes you, and for how long', group: 'sales', icon: Users },
  { to: '/admin/finance', title: 'Finance and P&L', desc: 'Revenue, cost, profit, dues and how the money came in', group: 'money', icon: Wallet },
  { to: '/admin/reports/profit-by-category', title: 'Profit by category', desc: 'Revenue, cost and profit per category', group: 'money', icon: FolderTree },
  { to: '/admin/reports/expenses', title: 'Expenses by category', desc: 'Where the running costs went', group: 'money', icon: Receipt },
  { to: '/admin/tills', title: 'Cash register', desc: 'Till sessions with over / short, totals and export', group: 'money', icon: Landmark },
  { to: '/admin/reports/tax', title: 'Tax', desc: 'Tax charged at the till, day by day', group: 'money', icon: Calculator, gate: 'tax' },
  { to: '/admin/reports/inventory', title: 'Inventory valuation', desc: 'Stock on hand at cost and at selling price', group: 'stock', icon: Boxes },
  { to: '/admin/stock/batches', title: 'Batches and expiry', desc: 'Live lots, what’s expired or close to it', group: 'stock', icon: CalendarClock },
  { to: '/admin/reports/count-sheet', title: 'Stock count sheet', desc: 'A printable shelf count by category or supplier', group: 'stock', icon: ClipboardList },
  { to: '/admin/reports/topup', title: 'Top-up and reorder', desc: 'What needs buying against the money on hand', group: 'stock', icon: Fuel },
  { to: '/admin/reports/purchases', title: 'Purchases', desc: 'Deliveries received, and what’s still to pay', group: 'stock', icon: PackageCheck },
  { to: '/admin/reports/supplier-spend', title: 'Spend by supplier', desc: 'Who you buy the most from', group: 'stock', icon: Truck },
  { to: '/admin/reports/supplier-dues', title: 'Supplier dues', desc: 'Who you owe, and what’s overdue', group: 'stock', icon: HandCoins },
  { to: '/admin/reorder', title: 'Low stock', desc: 'Items at or below their reorder level — order or print them', group: 'stock', icon: PackageSearch },
]

/** Not date-ranged reports — live pages people look for here (karots-pos "Live views"). */
export const LIVE: { to: string; title: string; desc: string; icon: LucideIcon }[] = [
  { to: '/admin/cashflow', title: 'Cash flow', desc: 'Live balances and money in / out', icon: ArrowLeftRight },
  { to: '/admin/receipts', title: 'Receipts', desc: 'Find and print any slip again', icon: ReceiptText },
]
