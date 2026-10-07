import { ArrowLeftRight, PackageSearch, ReceiptText, Wallet } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { Me } from '@/api/types.gen'

export type ReportGroup = 'sales' | 'money' | 'stock'
export type ReportEntry = { to: string; title: string; desc: string; group: ReportGroup; icon: LucideIcon; when?: (me: Me) => boolean }

/** karots-pos reportHubGroups: three groups, curated order (common first). Each
 * report adds itself here as it lands. */
export const GROUPS: [ReportGroup, string][] = [['sales', 'Sales and customers'], ['money', 'Money and profit'], ['stock', 'Inventory and suppliers']]

export const REPORTS: ReportEntry[] = [
  { to: '/admin/finance', title: 'Finance and P&L', desc: 'Revenue, cost, profit, dues and how the money came in', group: 'money', icon: Wallet },
  { to: '/admin/reorder', title: 'Low stock', desc: 'Items at or below their reorder level — order or print them', group: 'stock', icon: PackageSearch },
]

/** Not date-ranged reports — live pages people look for here (karots-pos "Live views"). */
export const LIVE: { to: string; title: string; desc: string; icon: LucideIcon }[] = [
  { to: '/admin/cashflow', title: 'Cash flow', desc: 'Live balances and money in / out', icon: ArrowLeftRight },
  { to: '/admin/receipts', title: 'Receipts', desc: 'Find and print any slip again', icon: ReceiptText },
]
