import { Pill } from 'lucide-react'
import type { slots as Slots } from '../slots'
import { DOSE_KIND } from './dose'
import { LineHint } from './LineHint'
import { PatientPanel } from './PatientPanel'
import { PharmacyPage } from './PharmacyPage'
import { ProductSection } from './ProductSection'

/** Pharmacy's React half: the Apps tile and its pages, the product form's Pharmacy
 * marks, the till's 💊 Patient panel, cart line reminders and the dose label's looks.
 * The patient on the sale (sale meta) goes with it: the server writes the dispense in the sale (plugins/pharmacy sold.go). */
export default function pharmacy(s: typeof Slots) {
  s.apps.push({ key: 'pharmacy', label: 'Pharmacy', description: 'Patients, prescriptions, refills and the controlled register', icon: Pill, color: 'var(--brand)', to: '/admin/x/pharmacy' })
  s.adminPages.push({ key: 'pharmacy', Page: PharmacyPage })
  s.productFormSections.push({ key: 'pharmacy', Section: ProductSection })
  s.tillActions.push({ key: 'pharmacy', label: '💊 Patient', Panel: PatientPanel })
  s.tillLineHints.push({ key: 'pharmacy', Hint: LineHint })
  s.labelTemplates.push(DOSE_KIND)
  s.reportCards.push({ href: '/admin/x/pharmacy/refills', title: 'Pharmacy — Refills due', desc: 'Patients about to run out of a medicine, to call or WhatsApp' })
}
