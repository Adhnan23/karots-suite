import { existsSync, readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { describe, expect, it } from 'vitest'
import type { SaleLine } from '@/api/types.gen'
import { shareOut } from '@/features/till/cart/totals'
import { refundOf } from './refund'

/** proofs/cases/<name>, found by walking up (tests also run from a copy, e.g. Stryker's sandbox). */
function proofCases(name: string) {
  for (let dir = process.cwd(); ; dir = dirname(dir)) {
    const f = join(dir, 'proofs', 'cases', name)
    if (existsSync(f)) return JSON.parse(readFileSync(f, 'utf8'))
    if (dirname(dir) === dir) throw new Error(`proofs/cases/${name} not found`)
  }
}

// The proven model (proofs/Karots/Refund.lean) wrote these cases; the return
// screen shows each refund to the cent, as the server pays it (RET-4, TILL-2).
const cases = proofCases('refunds.json') as
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

// proofs/Karots/Share.lean: the till shares a bill discount over lines as the
// server and the proven rule do (SALE-35).
const shares = proofCases('shares.json') as
  { amount: number; weights: number[]; shares: number[] }[]

describe('the till shares like the proof', () => {
  it(`all ${shares.length} cases`, () => {
    for (const c of shares) {
      expect(shareOut(c.amount / 100, c.weights.map((w) => w / 100)).map((v) => Math.round(v * 100)), JSON.stringify(c)).toEqual(c.shares)
    }
  })
})
