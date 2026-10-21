import { afterEach, describe, expect, it } from 'vitest'
import { dur, ease, tx } from './motion'

const root = document.documentElement

describe('motion tokens', () => {
  afterEach(() => root.removeAttribute('style'))

  it('reads durations in seconds', () => {
    root.style.setProperty('--dur-fast', '80ms')
    root.style.setProperty('--dur-slow', '0.3s')
    expect(dur('fast')).toBe(0.08)
    expect(dur('slow')).toBe(0.3)
  })
  it('parses the easing curve', () => {
    root.style.setProperty('--ease-spring', 'cubic-bezier(.2, 1.4, .4, 1)')
    expect(ease('spring')).toEqual([0.2, 1.4, 0.4, 1])
  })
  it('falls back when a token is missing', () => {
    expect(dur('base')).toBe(0.22)
    expect(tx('fast')).toEqual({ duration: 0.15, ease: [0.2, 0.8, 0.2, 1] })
  })
})
