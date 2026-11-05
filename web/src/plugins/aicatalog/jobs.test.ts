import { describe, expect, it } from 'vitest'
import { answersNeed, applyBody, batchLabel, byProduct, cleanupPart, initialTicks, rowKey, tickAll, type JobRow } from './jobs'

const row = (over: Partial<JobRow>): JobRow => ({ productId: 1, kind: 'hint', key: 'hint', label: 'Hint', old: '', new: 'x', reason: '', isNew: false, change: false, ticked: false, ...over })

describe('job rows', () => {
  it('starts ticked as the server says, never a row still needing something', () => {
    const rows = [row({ ticked: true }), row({ kind: 'name' }), row({ kind: 'dose', ticked: true, needs: 'Type the dose' })]
    expect([...initialTicks(rows)]).toEqual([rowKey(rows[0], 0)])
  })
  it('tick all covers one kind only, and not a one-by-one job', () => {
    const rows = [row({ kind: 'name' }), row({ kind: 'name', productId: 2 }), row({ kind: 'hint' }), row({ kind: 'name', productId: 3, needs: 'x' })]
    expect(tickAll(rows, new Set(), 'name', false).size).toBe(2)
    expect(tickAll(rows, new Set(), 'name', true).size).toBe(0)
  })
  it('groups rows under their product in order', () => {
    const g = byProduct([row({}), row({ productId: 2 }), row({ kind: 'name' })])
    expect(g.map((x) => [x.productId, x.rows.length])).toEqual([[1, 2], [2, 1]])
  })
  it('apply sends ticked rows and everyone who had a suggestion', () => {
    const rows = [row({}), row({ productId: 2 })]
    const b = applyBody({ rows, asked: [1, 2, 3], names: {}, remaining: 0 }, new Set([rowKey(rows[0], 0)]))
    expect(b.rows).toEqual([rows[0]])
    expect(b.asked).toEqual([1, 2, 3])
    expect(b.suggested).toEqual([1, 2])
    expect(b.open).toEqual([])
    expect(applyBody({ rows: [row({ needs: 'x' })], asked: [1], names: {}, remaining: 0 }, new Set()).open).toEqual([1])
  })
  it('a typed dose answers what a row needs; blank or "new" doesn\'t', () => {
    expect(answersNeed('1-0-0')).toBe(true)
    expect(answersNeed('  ')).toBe(false)
    expect(answersNeed('New')).toBe(false)
  })
  it('says where a run stands, also after coming back', () => {
    expect(batchLabel(0, 27, 5)).toBe('Batch 1 of 6: products 1–5 of 27')
    expect(batchLabel(5, 22, 5)).toBe('Batch 2 of 6: products 6–10 of 27')
    expect(batchLabel(25, 2, 5)).toBe('Batch 6 of 6: products 26–27 of 27')
    expect(batchLabel(1, 2, 1)).toBe('Batch 2 of 3: product 2 of 3')
    expect(batchLabel(0, 0, 5)).toBe('')
  })
  it('says which part of a clean-up this is', () => {
    expect(cleanupPart(0, 612)).toBe('Part 1 of 5: products 1–150 of 612')
    expect(cleanupPart(600, 612)).toBe('Part 5 of 5: products 601–612 of 612')
    expect(cleanupPart(0, 0)).toBe('')
  })
})
