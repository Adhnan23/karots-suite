import { Home, Palette } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

export type NavItem = {
  to: string
  label: string
  icon: LucideIcon
  mobile?: boolean
  systemOnly?: boolean
}

/** M0 holds only Home + the system-only Appearance entry; later milestones append. */
export const adminNav: NavItem[] = [
  { to: '/admin', label: 'Home', icon: Home, mobile: true },
  { to: '/admin/system/appearance', label: 'Appearance', icon: Palette, systemOnly: true },
]
