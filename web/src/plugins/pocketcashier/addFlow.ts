import type { CopyProduct, Lot, TillCopy } from '@/api/types.gen'
import { lotsFor } from '@/features/till/cart/lines'
import { num } from '@/lib/money'

export type AddStep =
  | { kind: 'add'; lots: Lot[]; units?: number }
  | { kind: 'price'; lots: Lot[]; units?: number }
  | { kind: 'pack'; lots: Lot[] }

/** The PC till's add decision (useAddProduct), over the copy so it works offline:
 * lots that disagree on price or an expired selling lot ask Which price?; a
 * product sold by the pack asks loose or which pack, unless a pack's own code was
 * scanned (packId) — then that pack is added. */
export function addFlow(copy: TillCopy, p: CopyProduct, packId?: number): AddStep {
  const lots = lotsFor(copy.priceOptions, p.id)
  const pack = packId ? p.packs.find((k) => k.id === packId) : undefined
  const units = pack ? num(pack.size) : undefined
  if (lots.length) return { kind: 'price', lots, units }
  if (!pack && p.packs.length && !p.isService) return { kind: 'pack', lots }
  return { kind: 'add', lots, units }
}
