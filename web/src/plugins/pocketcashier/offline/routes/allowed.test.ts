import { describe, expect, it } from 'vitest'
import allowed from '../../../../../../plugins/pocketcashier/call_routes.json'
import { alternativesRoutes } from './alternatives'
import { combosRoutes } from './combos'
import { coreRoutes } from './core'
import { documentsRoutes } from './documents'
import { pharmacyRoutes } from './pharmacy'
import { rechargeRoutes } from './recharge'
import { repairsRoutes } from './repairs'

describe('what the PC lets a phone replay', () => {
  const queued = [alternativesRoutes, combosRoutes, coreRoutes, documentsRoutes, pharmacyRoutes, rechargeRoutes, repairsRoutes].flat().filter((r) => r.queue)
  it('covers every write the phone queues', () => {
    expect(queued.length).toBeGreaterThan(10)
    for (const r of queued) expect(allowed).toContain(`${r.method} /api/v1${r.path}`)
  })
})
