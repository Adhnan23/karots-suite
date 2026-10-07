import { ArrowLeftRight, Clock, PackageSearch, ReceiptText, TrendingUp, Wallet } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

export type ReportGroup = 'sales' | 'money' | 'stock'
/** gate 'tax': only for a tax-registered shop (karots-pos hides the Tax report otherwise). */
export type ReportEntry = { to: string; title: string; desc: string; group: ReportGroup; icon: LucideIcon; gate?: 'tax' }

/** karots-pos reportHubGroups: three groups, curated order (common first). Each
 * report adds itself here as it lands. */
export const GROUPS: [ReportGroup, string][] = [['sales', 'Sales and customers'], ['money', 'Money and profit'], ['stock', 'Inventory and suppliers']]

export const REPORTS: ReportEntry[] = [
  { to: '/admin/reports/sales', title: 'Sales', desc: 'Receipts, profit and the trend, against the period before', group: 'sales', icon: TrendingUp },
  { to: '/admin/reports/peak-hours', title: 'Peak hours', desc: 'Busiest days and hours — plan staff and breaks', group: 'sales', icon: Clock },
  { to: '/admin/finance', title: 'Finance and P&L', desc: 'Revenue, cost, profit, dues and how the money came in', group: 'money', icon: Wallet },
  { to: '/admin/reorder', title: 'Low stock', desc: 'Items at or below their reorder level — order or print them', group: 'stock', icon: PackageSearch },
]

/** Not date-ranged reports — live pages people look for here (karots-pos "Live views"). */
export const LIVE: { to: string; title: string; desc: string; icon: LucideIcon }[] = [
  { to: '/admin/cashflow', title: 'Cash flow', desc: 'Live balances and money in / out', icon: ArrowLeftRight },
  { to: '/admin/receipts', title: 'Receipts', desc: 'Find and print any slip again', icon: ReceiptText },
]
