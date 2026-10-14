import type { slots as Slots } from '../slots'
import { LineHint } from './LineHint'
import { RepairsPanel } from './RepairsPanel'
import { afterSale, isTicket } from './till'

/** Repairs' React half: the till's 🔧 Repairs panel (a scanned ticket opens it),
 * job lines marked on the cart, jobs collected when their sale completes. */
export default function repairs(s: typeof Slots) {
  s.tillActions.push({ key: 'repairs', label: '🔧 Repairs', claimsText: isTicket, Panel: RepairsPanel })
  s.tillLineHints.push({ key: 'repairs', Hint: LineHint })
  s.tillAfterSale.push(afterSale)
}
