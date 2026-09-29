import { describe, expect, it } from 'vitest'
import type { CategoryNode } from '@/api/types.gen'
import { pathName, subtree, visibleTree } from './tree'

const n = (id: number, name: string, path: number[]): CategoryNode => ({ id, name, parentId: path.at(-1) ?? 0, depth: path.length, childCount: 0, path, productCount: 0 })
const nodes = [n(1, 'Food', []), n(2, 'Fruit', [1]), n(3, 'Apple', [1, 2]), n(4, 'Drinks', [])]

describe('category tree', () => {
  it('search keeps matches and their ancestors', () => {
    expect(visibleTree(nodes, 'app', new Set()).map((x) => x.name)).toEqual(['Food', 'Fruit', 'Apple'])
  })
  it('collapsing hides every descendant', () => {
    expect(visibleTree(nodes, '', new Set([1])).map((x) => x.name)).toEqual(['Food', 'Drinks'])
  })
  it('names a path and a subtree', () => {
    expect(pathName(nodes, 3)).toBe('Food › Fruit › Apple')
    expect([...subtree(nodes, 1)]).toEqual([1, 2, 3])
  })
})
