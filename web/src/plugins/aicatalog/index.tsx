import { Sparkles } from 'lucide-react'
import type { slots as Slots } from '../slots'
import { AssistantPage } from './AssistantPage'
import { ComboSuggest } from './ComboSuggest'
import { FieldTicks } from './FieldTicks'
import { HintSection } from './HintSection'
import { jobControls } from './jobControls'
import { Identify } from './Identify'
import { SuggestGroups } from './PageAction'

/** AI Assistant's React half: ✨ Identify under the product name (form + intake), and the Apps
 * page (Clean up, Settings). */
export default function aicatalog(s: typeof Slots) {
  s.apps.push({ key: 'aicatalog', label: 'AI Assistant', description: 'Identify products and tidy the catalogue', icon: Sparkles, color: 'var(--brand)', to: '/admin/x/aicatalog' })
  s.adminPages.push({ key: 'aicatalog', Page: AssistantPage })
  s.productAssists.push({ key: 'aicatalog', Assist: Identify })
  s.productFormSections.push({ key: 'aicatalog', Section: HintSection })
  s.pageActions.push({ key: 'aicatalog', page: 'alternatives', Action: SuggestGroups })
  s.pageActions.push({ key: 'aicatalog', page: 'combos', Action: ComboSuggest })
  jobControls.fields = FieldTicks
}
