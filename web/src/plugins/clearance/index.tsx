import { Tags } from 'lucide-react'
import type { slots as Slots } from '../slots'
import { ClearancePage } from './ClearancePage'

/** Clearance's React half: the Apps tile and its admin page. The till badge,
 * apply / skip prompt and ⓘ row come from its Go hooks through core's product data. */
export default function clearance(s: typeof Slots) {
  s.apps.push({ key: 'clearance', label: 'Clearance', description: 'Mark down slow and expiring stock', icon: Tags, color: 'var(--brand)', to: '/admin/x/clearance' })
  s.adminPages.push({ key: 'clearance', Page: ClearancePage })
}
