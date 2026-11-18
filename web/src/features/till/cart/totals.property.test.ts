import fc from 'fast-check'
import { describe, expect, it } from 'vitest'
import type { CartLine } from './types'
import { total } from './totals'

// TILL-1: the total the till shows = the total the server charges, to the cent,
// for every cart. The reference below is the rule book's pricing (docs/rules/
// sales.md) in whole numbers — cents, thousandths of a unit, tenths of a percent —
// rounding half up at the server's places: each line's gross, each discount, a
// percent bill discount, each line's tax on its net less its share of the bill
// discount (SALE-35).

/** a ÷ d rounded half up, a ≥ 0 (whole numbers). */
const half = (a: number, d: number) => Math.floor((2 * a + d) / (2 * d))

type Gen = { priceC: number; qtyM: number; disc: '' | 'fixed' | 'percent'; discV: number; rateH: number }

function reference(ls: Gen[], billType: 'fixed' | 'percent', billV: number) {
  const nets = ls.map((l) => {
    const gross = half(l.qtyM * l.priceC, 1000)
    const d = l.disc === 'fixed' ? half(l.discV * l.qtyM, 1000) : l.disc === 'percent' ? half(gross * l.discV, 1000) : 0
    return gross - Math.min(Math.max(d, 0), gross)
  })
  const net = nets.reduce((a, b) => a + b, 0)
  const off = Math.min(billType === 'percent' ? half(net * billV, 1000) : billV, net)
  // SALE-35: the bill discount shared over the lines to the cent (largest remainder,
  // ties to the earlier line); tax on what each line is then paid.
  const share = nets.map((n) => (net > 0 ? Math.floor((n * off) / net) : 0))
  let left = off - share.reduce((a, b) => a + b, 0)
  const order = nets.flatMap((n, i) => (n > 0 ? [{ i, rem: (n * off) % net }] : [])).sort((a, b) => b.rem - a.rem || a.i - b.i)
  for (let k = 0; left > 0; k = (k + 1) % order.length, left--) share[order[k].i]++
  const tax = nets.reduce((t, n, i) => t + half((n - share[i]) * ls[i].rateH, 10000), 0)
  return net - off + tax
}

const toLine = (g: Gen, i: number): CartLine => ({
  key: i, productId: i, name: 'x', unitPrice: g.priceC / 100, taxRate: g.rateH / 100, qty: g.qtyM / 1000, stock: 1e9,
  isService: false, allowDecimal: true, batchId: 0, batchRemaining: 0, batchLabel: '', allowOversell: false,
  discount: g.disc === 'fixed' ? (g.discV / 100).toFixed(2) : g.disc === 'percent' ? (g.discV / 10).toFixed(1) : '',
  discountType: g.disc === 'percent' ? 'percent' : 'fixed', trackSerial: false, warrantyMonths: 0, serials: [],
  priceOverride: '', description: '', components: [],
})

const genLine = fc.record({
  priceC: fc.integer({ min: 1, max: 2_000_000 }),
  qtyM: fc.oneof(fc.integer({ min: 1, max: 30 }).map((n) => n * 1000), fc.integer({ min: 1, max: 50_000 })),
  disc: fc.constantFrom('' as const, 'fixed' as const, 'percent' as const),
  discV: fc.integer({ min: 0, max: 1000 }),
  rateH: fc.constantFrom(0, 0, 800, 1250, 1800),
})

describe('the till total = the server total (TILL-1)', () => {
  it('for any cart, item discounts and bill discount', () => {
    fc.assert(
      fc.property(fc.array(genLine, { minLength: 1, maxLength: 8 }), fc.constantFrom('fixed' as const, 'percent' as const), fc.integer({ min: 0, max: 100_000 }),
        (ls, billType, b) => {
          const billV = billType === 'percent' ? b % 1001 : b
          const want = reference(ls, billType, billV) / 100
          const value = billType === 'percent' ? (billV / 10).toFixed(1) : (billV / 100).toFixed(2)
          expect(total(ls.map(toLine), value, billType)).toBe(want)
        }),
      { numRuns: 5000 },
    )
  })
})
