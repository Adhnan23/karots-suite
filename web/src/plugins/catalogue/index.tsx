import { BookOpenCheck } from 'lucide-react'
import type { slots as Slots } from '../slots'
import { CataloguePage } from './CataloguePage'
import { FromCatalogue } from './FromCatalogue'
import { ReviewButtons } from './ReviewButtons'

/** Catalogue's React half: the Apps tile and its page, and From catalogue under the name on the product form,
 * intake, the till's quick item and a delivery's new product line. */
export default function catalogue(s: typeof Slots) {
  s.apps.push({ key: 'catalogue', label: 'Catalogue', description: 'Pick products from the Karots Suite catalogue', icon: BookOpenCheck, color: 'var(--brand)', to: '/admin/x/catalogue' })
  s.adminPages.push({ key: 'catalogue', Page: CataloguePage })
  s.productAssists.push({ key: 'catalogue', Assist: FromCatalogue })
  s.reviewActions.push({ key: 'catalogue', Action: ReviewButtons })
}
