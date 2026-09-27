import { beforeEach, describe, expect, it } from 'vitest'
import { getMode, setMode } from './mode'

describe('mode', () => {
  beforeEach(() => localStorage.clear())
  it('defaults to system and sets no data-theme', () => {
    const el = document.createElement('div')
    expect(getMode()).toBe('system')
    setMode('system', el)
    expect(el.dataset.theme).toBeUndefined()
  })
  it('persists light/dark and applies data-theme', () => {
    const el = document.createElement('div')
    setMode('dark', el)
    expect(getMode()).toBe('dark')
    expect(el.dataset.theme).toBe('dark')
    setMode('system', el)
    expect(el.dataset.theme).toBeUndefined()
  })
  it('ignores junk in storage', () => {
    localStorage.setItem('ks-mode', 'purple')
    expect(getMode()).toBe('system')
  })
})
