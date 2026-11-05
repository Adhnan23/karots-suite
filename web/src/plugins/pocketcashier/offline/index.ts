import { addRoutes } from './fetch'
import { alternativesRoutes } from './routes/alternatives'
import { combosRoutes } from './routes/combos'
import { coreRoutes } from './routes/core'
import { documentsRoutes } from './routes/documents'
import { pharmacyRoutes } from './routes/pharmacy'
import { rechargeRoutes } from './routes/recharge'

let added = false

/** Every call the phone can answer itself. A plugin that isn't compiled in never
 * makes its calls, so its routes simply never match. */
export function setupOfflineRoutes() {
  if (added) return
  added = true
  addRoutes(coreRoutes)
  addRoutes(rechargeRoutes)
  addRoutes(documentsRoutes)
  addRoutes(combosRoutes)
  addRoutes(alternativesRoutes)
  addRoutes(pharmacyRoutes)
}
