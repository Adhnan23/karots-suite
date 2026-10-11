import type { CartLine } from '@/features/till/cart/types'
import type { Combo } from './types'

/** A line a combo may draw from: plain, full price, FEFO, not a service. */
export const plain = (l: CartLine) => !l.group && !(Number(l.discount) > 0) && !l.batchId && !l.isService

const have = (lines: CartLine[], productId: number) =>
  lines.filter((l) => l.productId === productId && plain(l)).reduce((s, l) => s + l.qty, 0)

/** Units to take off plain lines for `units` of each member (first lines first). */
function takeFor(c: Combo, lines: CartLine[], sets: number) {
  const take: { key: number; qty: number }[] = []
  for (const m of c.members) {
    let need = Number(m.qty) * sets
    for (const l of lines) {
      if (need <= 0) break
      if (l.productId !== m.productId || !plain(l)) continue
      const q = Math.min(need, l.qty)
      take.push({ key: l.key, qty: q })
      need -= q
    }
  }
  return take
}

export type Hint = { combo: Combo; sets: number; take: { key: number; qty: number }[] }

/** Combos the cart holds full sets of, as plain lines. */
export function findHints(combos: Combo[], lines: CartLine[]): Hint[] {
  const out: Hint[] = []
  for (const c of combos) {
    if (c.problem || !c.members.length) continue
    const sets = Math.min(...c.members.map((m) => Math.floor(have(lines, m.productId) / Number(m.qty))))
    if (sets > 0) out.push({ combo: c, sets, take: takeFor(c, lines, sets) })
  }
  return out
}

export type Partial = { combo: Combo; take: { key: number; qty: number }[]; missing: string[] }

/** Combos the cart holds some members of but not one full set: what's there is
 * taken, the rest is added. */
export function findPartials(combos: Combo[], lines: CartLine[]): Partial[] {
  const out: Partial[] = []
  for (const c of combos) {
    if (c.problem || !c.members.length) continue
    const got = c.members.map((m) => Math.min(have(lines, m.productId), Number(m.qty)))
    if (got.every((g, i) => g >= Number(c.members[i].qty)) || got.every((g) => g === 0)) continue
    const take: Partial['take'] = []
    c.members.forEach((m, i) => {
      let need = got[i]
      for (const l of lines) {
        if (need <= 0) break
        if (l.productId !== m.productId || !plain(l)) continue
        const q = Math.min(need, l.qty)
        take.push({ key: l.key, qty: q })
        need -= q
      }
    })
    const missing = c.members.flatMap((m, i) => {
      const n = Number(m.qty) - got[i]
      return n > 0 ? [n > 1 ? `${n} × ${m.name}` : m.name] : []
    })
    out.push({ combo: c, take, missing })
  }
  return out
}
