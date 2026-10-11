import { describe, expect, it } from 'vitest'
import { askLabel } from './labels'

const now = new Date('2026-10-11T15:00:00+05:30')
const daysAgo = (n: number) => new Date(now.getTime() - n * 86_400_000).toISOString()

describe('askLabel', () => {
  it('counts the asks and says when the last one was', () => {
    expect(askLabel(3, daysAgo(2), now)).toBe('Asked 3× · last asked 2 days ago')
  })
  it('a single ask today', () => {
    expect(askLabel(1, daysAgo(0), now)).toBe('Asked today')
  })
  it('yesterday', () => {
    expect(askLabel(2, daysAgo(1), now)).toBe('Asked 2× · last asked yesterday')
  })
})
