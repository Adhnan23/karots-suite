import { lazy, Suspense } from 'react'
import type { slots as Slots } from '../slots'

const App = lazy(() => import('./App'))

/** Pocket Cashier's React half: the phone app (a surface at /x/pocketcashier)
 * and the admin Phones page. */
export default function pocketcashier(s: typeof Slots) {
  s.surfaces.push({ key: 'pocketcashier', Surface: (p) => <Suspense fallback={null}><App {...p} /></Suspense> })
}
