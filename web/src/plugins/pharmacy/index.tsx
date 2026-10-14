import { Pill } from 'lucide-react'
import type { slots as Slots } from '../slots'
import { PharmacyPage } from './PharmacyPage'
import { ProductSection } from './ProductSection'

/** Pharmacy's React half: the Apps tile and its pages, the product form's Pharmacy marks. */
export default function pharmacy(s: typeof Slots) {
  s.apps.push({ key: 'pharmacy', label: 'Pharmacy', description: 'Patients, prescriptions, refills and the controlled register', icon: Pill, color: 'var(--brand)', to: '/admin/x/pharmacy' })
  s.adminPages.push({ key: 'pharmacy', Page: PharmacyPage })
  s.productFormSections.push({ key: 'pharmacy', Section: ProductSection })
}
