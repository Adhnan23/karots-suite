import { Smartphone, SmartphoneNfc } from 'lucide-react'
import type { slots as Slots } from '../slots'
import { DrawerCounts } from './DrawerCounts'
import { FloatJob } from './FloatJob'
import { LineHint } from './LineHint'
import { RechargePage } from './RechargePage'
import { ReturnField } from './ReturnField'
import { claims } from './till'
import { TillPanel } from './TillPanel'
import { WalletTender } from './WalletTender'

/** Reload & Bills' React half: the till's 📲 menu (reloads, packages, bills, wallet
 * deposits, cash outs — all cart lines on the normal Pay), float counts in the drawer
 * dialogs, the wallet tender, the Returns "how much came back?",
 * and Apps → Reload & Bills. */
export default function recharge(s: typeof Slots) {
  s.tillMenu.push({ key: 'recharge', label: 'Reload & Bills', emoji: '📲', claims, Panel: TillPanel })
  s.tillLineHints.push({ key: 'recharge', Hint: LineHint })
  s.drawerSections.push({ key: 'recharge', on: ['close'], Section: DrawerCounts })
  s.tillTenders.push({ value: 'wallet', Icon: SmartphoneNfc, Detail: WalletTender })
  s.supplierJobs.push({ key: 'recharge-float', label: '📲 Float', Panel: FloatJob })
  s.returnLineFields.push({ key: 'recharge', Field: ReturnField })
  s.apps.push({ key: 'recharge', label: 'Reload & Bills', description: 'Floats, carriers, packages and commission', icon: Smartphone, color: 'var(--brand)', to: '/admin/x/recharge' })
  s.adminPages.push({ key: 'recharge', Page: RechargePage })
  s.reportCards.push({ href: '/admin/x/recharge/report', title: 'Reload & Bills', desc: 'Earnings by carrier, bills, and float counts' })
}
