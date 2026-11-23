import type { CartLine } from '@/features/till/cart/types'
import type { Group, Tier } from './types'

/** A group shows tiers once one is named or there are two; else it's a plain list. */
export const hasTiers = (g: Group) => g.tiers.length > 1 || (g.tiers[0]?.name ?? '') !== ''

/** Saving a tier keeps it on Auto unless a different number was typed (a rename stays Auto). */
export const staysAuto = (t: Tier, level: number) => t.levelAuto && level === t.reorderLevel

/** A plain cart line holding more than is in stock — the swap chip's cue. */
export const isShort = (l: CartLine) => !l.group && !l.isService && l.qty > l.stock
