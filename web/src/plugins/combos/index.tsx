import { Gift } from 'lucide-react'
import type { slots as Slots } from '../slots'
import { CartHint, LineHint } from './CartHint'
import { CombosPage } from './CombosPage'
import { ComboSettings } from './Settings'
import { afterSale } from './till'
import { TillPanel } from './TillPanel'

/** Combos' React half: the Apps tile and its page (the report at …/report), the
 * till's 🎁 Combos menu, the cart hints (or auto apply, a shop setting) and the sold record after each sale. */
export default function combos(s: typeof Slots) {
  s.apps.push({ key: 'combos', label: 'Combos', description: 'Product deals rung in one tap', icon: Gift, color: 'var(--brand)', to: '/admin/x/combos' })
  s.adminPages.push({ key: 'combos', Page: CombosPage })
  s.tillMenu.push({ key: 'combos', label: 'Combos', emoji: '🎁', Panel: TillPanel })
  s.tillCartHints.push({ key: 'combos', Hint: CartHint })
  s.tillLineHints.push({ key: 'combos', Hint: LineHint })
  s.tillAfterSale.push(afterSale)
  s.settingsSections.push({ key: 'combos', title: 'Combos', Section: ComboSettings })
  s.reportCards.push({ href: '/admin/x/combos/report', title: 'Combos', desc: 'Sets sold, sales and discount given per combo' })
}
