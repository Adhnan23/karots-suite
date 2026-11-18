import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import type { SaleLine } from '@/api/types.gen'
import { refundOf } from './refund'

// The proven model (proofs/Karots/Refund.lean) wrote these cases; the return
// screen shows each refund to the cent, as the server pays it (RET-4, TILL-2).
const cases = JSON.parse(readFileSync(`${process.cwd()}/../proofs/cases/refunds.json`, 'utf8')) as
  { paid: number; qty: number; before: number; back: number; refund: number }[]

describe('the return screen agrees with the proof', () => {
  it(`all ${cases.length} cases`, () => {
    expect(cases.length).toBeGreaterThan(1000)
    for (const c of cases) {
      if (c.before + c.back >= c.qty) continue // the last of a line is refundLeft, from the server
      const l = { quantity: String(c.qty / 1000), returnedQty: String(c.before / 1000), paid: (c.paid / 100).toFixed(2), subtotal: '1', refundLeft: '0' } as SaleLine
      expect(Math.round(refundOf(l, c.back / 1000) * 100), JSON.stringify(c)).toBe(c.refund)
    }
  })
})
