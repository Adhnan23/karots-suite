import { RouteError, type OfflineRoute } from '../fetch'

/** The catalogue's copy lives on the main PC only (CTL-5): offline, From catalogue says so at once and the
 * person types the product in (CTL-4). */
const away = () => { throw new RouteError('The catalogue is on the PC') }

export const catalogueRoutes: OfflineRoute[] = [
  { method: 'GET', path: '/x/catalogue/status', answer: away },
  { method: 'GET', path: '/x/catalogue/search', answer: away },
]
