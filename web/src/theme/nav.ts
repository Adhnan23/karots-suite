import { BarChart3, Package, ShoppingBag, ShoppingCart, Wallet, Wrench } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

export type NavPage = { to: string; label: string; description: string; badge?: number }
export type NavArea = { key: string; label: string; icon: LucideIcon; pages: NavPage[] }
export type AppEntry = { key: string; label: string; description: string; icon: LucideIcon; color: string }

/** karots-pos's admin areas (templates/layouts/admin.templ). Each area's `pages`
 * fills in as its milestone ports it; M0 has none, so the dock only shows Home,
 * Apps and the till link — no shelf opens for an area with nothing in it yet. */
export const adminNav: NavArea[] = [
  { key: 'sell', label: 'Sell', icon: ShoppingCart, pages: [] },
  { key: 'stock', label: 'Inventory', icon: Package, pages: [] },
  { key: 'buy', label: 'Purchasing', icon: ShoppingBag, pages: [] },
  { key: 'money', label: 'Money', icon: Wallet, pages: [] },
  { key: 'reports', label: 'Reports', icon: BarChart3, pages: [] },
  { key: 'setup', label: 'Setup', icon: Wrench, pages: [] },
]

/** Plugin slot for the dock's Apps tile (theme.md §6). Empty until M7. */
export const apps: AppEntry[] = []
