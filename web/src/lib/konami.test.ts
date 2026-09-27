import { describe, expect, it, vi } from 'vitest'
import { KONAMI, konamiMatcher } from './konami'

describe('konami', () => {
  it('fires once on the full sequence', () => {
    const hit = vi.fn()
    const feed = konamiMatcher(hit)
    KONAMI.forEach(feed)
    expect(hit).toHaveBeenCalledTimes(1)
  })
  it('accepts caps-lock B/A', () => {
    const hit = vi.fn()
    const feed = konamiMatcher(hit)
    ;[...KONAMI.slice(0, 8), 'B', 'A'].forEach(feed)
    expect(hit).toHaveBeenCalledTimes(1)
  })
  it('recovers after a wrong key', () => {
    const hit = vi.fn()
    const feed = konamiMatcher(hit)
    ;['ArrowUp', 'ArrowUp', 'x', ...KONAMI].forEach(feed)
    expect(hit).toHaveBeenCalledTimes(1)
  })
  it('tolerates an extra ArrowUp before the sequence (karots-pos matcher failed this)', () => {
    const hit = vi.fn()
    const feed = konamiMatcher(hit)
    ;['ArrowUp', ...KONAMI].forEach(feed)
    expect(hit).toHaveBeenCalledTimes(1)
  })
})
