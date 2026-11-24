import { BookOpenCheck } from 'lucide-react'
import type { slots as Slots } from '../slots'
import { CataloguePage } from './CataloguePage'

/** Catalogue's React half: the Apps tile and its page. */
export default function catalogue(s: typeof Slots) {
  s.apps.push({ key: 'catalogue', label: 'Catalogue', description: 'Pick products from the Karots Suite catalogue', icon: BookOpenCheck, color: 'var(--brand)', to: '/admin/x/catalogue' })
  s.adminPages.push({ key: 'catalogue', Page: CataloguePage })
}
