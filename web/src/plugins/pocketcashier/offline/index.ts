import { addRoutes } from './fetch'
import { coreRoutes } from './routes/core'

let added = false

/** Every call the phone can answer itself. A plugin that isn't compiled in never
 * makes its calls, so its routes simply never match. */
export function setupOfflineRoutes() {
  if (added) return
  added = true
  addRoutes(coreRoutes)
}
