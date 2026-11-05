import { describe, expect, it } from 'vitest'
import type { ReorderRow } from '@/api/types.gen'
import { blocks } from './blocks'

const row = (productId: number, block?: string, blockLine?: string, blockKey?: string) => ({ productId, block, blockLine, blockKey }) as ReorderRow

describe('low-stock blocks', () => {
  it('keeps plain rows alone and in order', () => {
    expect(blocks([row(1), row(2)]).map((b) => [b.name, b.rows.map((r) => r.productId)])).toEqual([['', [1]], ['', [2]]])
  })
  it('gathers a block at its first row, with its line', () => {
    const got = blocks([row(1), row(2, 'Twins', 'LOW'), row(3), row(4, 'Twins', 'LOW')])
    expect(got.map((b) => b.rows.map((r) => r.productId))).toEqual([[1], [2, 4], [3]])
    expect(got[1].line).toBe('LOW')
  })
  it('two groups with one name stay apart', () => {
    expect(blocks([row(1, 'Twins', 'A', 't1'), row(2, 'Twins', 'B', 't2')]).map((b) => b.rows.length)).toEqual([1, 1])
  })
  it('handles nothing', () => {
    expect(blocks([])).toEqual([])
  })
})
