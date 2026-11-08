import type { Unit } from '@/api/types.gen'

/** The unit new products start with: the shop's default (Units page), else the first. */
export const defaultUnit = (units: Unit[]): Unit | undefined => units.find((u) => u.isDefault) ?? units[0]
