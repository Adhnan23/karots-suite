import { afterEach, describe, expect, it } from 'vitest'
import { pluginOn } from './active'

function meta(content: string) {
  const m = document.createElement('meta')
  m.name = 'ks-plugins'
  m.content = content
  document.head.appendChild(m)
}

describe('pluginOn', () => {
  afterEach(() => document.head.querySelectorAll('meta[name="ks-plugins"]').forEach((m) => m.remove()))

  it('is on for every plugin when the page has no list (a shop build)', () => {
    expect(pluginOn('x')).toBe(true)
  })
  it('follows the demo list', () => {
    meta('a,b')
    expect(pluginOn('a')).toBe(true)
    expect(pluginOn('b')).toBe(true)
    expect(pluginOn('c')).toBe(false)
  })
  it('an empty list turns every plugin off', () => {
    meta('')
    expect(pluginOn('a')).toBe(false)
  })
})
