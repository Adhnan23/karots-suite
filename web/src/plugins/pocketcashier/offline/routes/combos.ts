import type { OfflineRoute } from '../fetch'

type Part = { till: unknown[]; settings: unknown }

export const combosRoutes: OfflineRoute[] = [
  { method: 'GET', path: '/x/combos/till', answer: (c) => (c.parts.combos as Part | undefined)?.till ?? [] },
  { method: 'GET', path: '/x/combos/settings', answer: (c) => (c.parts.combos as Part | undefined)?.settings ?? { autoApply: false } },
  { method: 'POST', path: '/x/combos/sold', queue: true, label: () => 'Combos sold', answer: () => ({}) },
]
