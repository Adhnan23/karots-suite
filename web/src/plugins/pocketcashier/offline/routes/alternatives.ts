import type { OfflineRoute } from '../fetch'

type Part = { byProduct: Record<string, unknown[]> }

export const alternativesRoutes: OfflineRoute[] = [
  { method: 'GET', path: '/x/alternatives/till/:pid', answer: (c) => (c.parts.alternatives as Part | undefined)?.byProduct[c.params.pid] ?? [] },
]
