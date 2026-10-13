import type { slots as Slots } from '../slots'
import { Identify } from './Identify'

/** AI Assistant's React half: ✨ Identify under the product name (form + intake). */
export default function aicatalog(s: typeof Slots) {
  s.productAssists.push({ key: 'aicatalog', Assist: Identify })
}
