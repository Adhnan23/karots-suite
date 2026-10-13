import { Sparkles } from 'lucide-react'
import type { slots as Slots } from '../slots'
import { AssistantPage } from './AssistantPage'
import { Identify } from './Identify'

/** AI Assistant's React half: ✨ Identify under the product name (form + intake), and the Apps
 * page (Clean up, Settings). */
export default function aicatalog(s: typeof Slots) {
  s.apps.push({ key: 'aicatalog', label: 'AI Assistant', description: 'Identify products and tidy the catalogue', icon: Sparkles, color: 'var(--brand)', to: '/admin/x/aicatalog' })
  s.adminPages.push({ key: 'aicatalog', Page: AssistantPage })
  s.productAssists.push({ key: 'aicatalog', Assist: Identify })
}
