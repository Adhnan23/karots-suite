import { Printer } from 'lucide-react'
import type { slots as Slots } from '../slots'
import { PrintPage } from './PrintPage'
import { afterSale, claims } from './till'
import { TillPanel } from './TillPanel'

/** Print & Copy's React half: the Apps tile and its pages, the till's 🖨 menu (also
 * opened by picking a print service anywhere), the job ledger after each sale and
 * the report card. */
export default function documents(s: typeof Slots) {
  s.apps.push({ key: 'documents', label: 'Print & Copy', description: 'Copy, print, laminate and bind jobs', icon: Printer, color: 'var(--brand)', to: '/admin/x/documents' })
  s.adminPages.push({ key: 'documents', Page: PrintPage })
  s.tillMenu.push({ key: 'documents', label: 'Print & Copy', emoji: '🖨', claims, Panel: TillPanel })
  s.tillAfterSale.push(afterSale)
  s.reportCards.push({ href: '/admin/x/documents/report', title: 'Print & Copy', desc: 'Copies, sheets and sales per size, colour and side' })
}
