import fc from 'fast-check'
import { describe, expect, it } from 'vitest'
import type { Payment } from './types'
import { actualChange, changeDue, coinKeep, keptAmount, LKR_NOTES, notesFor, quickCash, tenderIssue } from './totals'

// TEND-2, TEND-3: the change maths, for any bill and any cash handed over.
const cents = fc.integer({ min: 0, max: 10_000_000 })
const cash = (c: number): Payment => ({ method: 'cash', amount: (c / 100).toFixed(2), reference: '', accountId: '', meta: {} })

describe('change (TEND-2, TEND-3)', () => {
  it('change = cash − bill to the cent; kept + handed back = change, kept never more', () => {
    fc.assert(fc.property(cents, cents, cents, (t, extra, keepC) => {
      const change = changeDue(t / 100, [cash(t + extra)])
      expect(Math.round(change * 100)).toBe(extra)
      const kept = keptAmount((keepC / 100).toFixed(2), change)
      expect(kept).toBeLessThanOrEqual(change)
      expect(Math.round((kept + actualChange(change, kept)) * 100)).toBe(extra)
      const coins = coinKeep(change)
      expect(coins).toBeGreaterThanOrEqual(0)
      expect(coins).toBeLessThan(10)
      expect(Math.round((change - coins) * 100) % 1000).toBe(0)
    }), { numRuns: 5000 })
  })

  it('the notes suggested never add up to more than the change, and leave less than the smallest', () => {
    fc.assert(fc.property(cents, (c) => {
      const ns = notesFor(c / 100, LKR_NOTES)
      const sumC = ns.reduce((s, n) => s + Math.round(n.value * 100) * n.qty, 0)
      expect(sumC).toBeLessThanOrEqual(c)
      expect(c - sumC).toBeLessThan(2000)
    }), { numRuns: 5000 })
  })

  it('quick cash: exact first, every chip covers the bill, smallest first, at most 8', () => {
    fc.assert(fc.property(fc.integer({ min: 1, max: 5_000_000 }), (c) => {
      const due = c / 100
      const qc = quickCash(due)
      expect(qc[0]).toBe(due)
      expect(qc.length).toBeLessThanOrEqual(8)
      qc.forEach((v, i) => {
        expect(v).toBeGreaterThanOrEqual(due)
        if (i > 0) expect(v).toBeGreaterThan(qc[i - 1])
      })
    }), { numRuns: 3000 })
  })

  it('a tender is short exactly when it covers less than the bill (TEND-1)', () => {
    fc.assert(fc.property(cents, cents, (t, paid) => {
      const issue = tenderIssue(t / 100, paid / 100, 0, false)
      expect(issue?.kind === 'shortfall').toBe(paid < t)
      if (issue) expect(Math.round(issue.amount * 100)).toBe(t - paid)
    }), { numRuns: 5000 })
  })
})
