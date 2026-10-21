import { Wrench } from 'lucide-react'
import type { slots as Slots } from '../slots'
import { JobSlip } from './JobSlip'
import { LineHint } from './LineHint'
import { RepairsPage } from './RepairsPage'
import { RepairsPanel } from './RepairsPanel'
import { afterSale, isTicket } from './till'

/** Repairs' React half: Apps → Repairs (jobs, profit, warranty), the till's 🔧 Repairs panel (a scanned ticket opens it),
 * job lines marked on the cart, jobs collected when their sale completes. */
export default function repairs(s: typeof Slots) {
  s.tillActions.push({ key: 'repairs', label: '🔧 Repairs', claimsText: isTicket, Panel: RepairsPanel })
  s.tillLineHints.push({ key: 'repairs', Hint: LineHint })
  s.tillAfterSale.push(afterSale)
  s.slipTemplates.push({ kind: 'repairs.job', Template: JobSlip })
  s.apps.push({ key: 'repairs', label: 'Repairs', description: 'Repair jobs, repairer payouts and profit', icon: Wrench, color: 'var(--brand)', to: '/admin/x/repairs' })
  s.adminPages.push({ key: 'repairs', Page: RepairsPage })
  s.reportCards.push({ href: '/admin/x/repairs/report', title: 'Repairs — profit', desc: 'Each collected repair: what it took, parts cost, the repairer, profit' })
}
