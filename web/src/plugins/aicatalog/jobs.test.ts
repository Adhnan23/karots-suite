import { describe, expect, it } from 'vitest'
import { applyBody, byProduct, initialTicks, rowKey, tickAll, type JobRow } from './jobs'

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
  })
})
