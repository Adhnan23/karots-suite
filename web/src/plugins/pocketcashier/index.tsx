import { Smartphone } from 'lucide-react'
import { lazy, Suspense } from 'react'
import type { slots as Slots } from '../slots'
import { PhonesPage } from './admin/PhonesPage'

const App = lazy(() => import('./App'))

/** Pocket Cashier's React half: the phone app (a surface at /x/pocketcashier)
 * and the admin Phones page. */
export default function pocketcashier(s: typeof Slots) {
  s.surfaces.push({ key: 'pocketcashier', Surface: (p) => <Suspense fallback={null}><App {...p} /></Suspense> })
  s.apps.push({ key: 'pocketcashier', label: 'Phones', description: 'Pocket Cashier phones and what they sent', icon: Smartphone, color: 'var(--brand)', to: '/admin/x/pocketcashier' })
  s.adminPages.push({ key: 'pocketcashier', Page: PhonesPage })
}
