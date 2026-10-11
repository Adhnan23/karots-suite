import { ListPlus } from 'lucide-react'
import type { slots as Slots } from '../slots'
import { FieldsPage } from './FieldsPage'
import { FieldsSection } from './FieldsSection'

/** Product Plus' React half: the Apps tile, its page and the product form section. */
export default function productplus(s: typeof Slots) {
  s.apps.push({ key: 'productplus', label: 'Product Plus', description: 'Extra fields for your products', icon: ListPlus, color: 'var(--brand)', to: '/admin/x/productplus' })
  s.adminPages.push({ key: 'productplus', Page: FieldsPage })
  s.productFormSections.push({ key: 'productplus', Section: FieldsSection })
}
