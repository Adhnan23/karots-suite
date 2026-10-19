import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { _reset, noteVersion, setReloadGuard, shouldReload, tillQuiet } from './version'

describe('shouldReload', () => {
  it('only for a changed version the guard allows', () => {
    expect(shouldReload(null, '2026.10.19.1530', () => true)).toBe(false)
    expect(shouldReload('2026.10.19.1530', '2026.10.19.1530', () => true)).toBe(false)
    expect(shouldReload('2026.10.19.1530', null, () => true)).toBe(false)
    expect(shouldReload('2026.10.19.1530', '2026.10.20.0900', () => true)).toBe(true)
    expect(shouldReload('2026.10.19.1530', '2026.10.20.0900', () => false)).toBe(false)
  })
})

describe('noteVersion', () => {
  let reloads = 0
  beforeEach(() => {
    vi.useFakeTimers()
    reloads = 0
    _reset(() => { reloads++ })
  })
  afterEach(() => vi.useRealTimers())

  it('remembers the first version and ignores repeats and missing headers', () => {
    noteVersion('2026.10.19.1530')
    noteVersion('2026.10.19.1530')
    noteVersion(null)
    vi.advanceTimersByTime(60_000)
    expect(reloads).toBe(0)
  })

  it('reloads after the banner when the version changes', () => {
    noteVersion('2026.10.19.1530')
    noteVersion('2026.10.20.0900')
    expect(reloads).toBe(0) // the banner shows first
    vi.advanceTimersByTime(1_600)
    expect(reloads).toBe(1)
  })

  it('waits while the till has a sale going, then reloads once it is empty', () => {
    let cartEmpty = false
    const off = setReloadGuard(() => cartEmpty)
    noteVersion('2026.10.19.1530')
    noteVersion('2026.10.20.0900')
    vi.advanceTimersByTime(30_000)
    expect(reloads).toBe(0)
    cartEmpty = true
    vi.advanceTimersByTime(6_600) // the next 5 s retry, then the banner
    expect(reloads).toBe(1)
    off()
  })
})

describe('tillQuiet', () => {
  it('only on the Sell screen with an empty cart and nothing open', () => {
    expect(tillQuiet('/till', true, false)).toBe(true)
    expect(tillQuiet('/till/', true, false)).toBe(true)
    expect(tillQuiet('/till', false, false)).toBe(false) // a sale on the counter
    expect(tillQuiet('/till', true, true)).toBe(false) // Close drawer, Pay… open
    expect(tillQuiet('/till/returns', true, false)).toBe(false) // a counter tab mid-task
    expect(tillQuiet('/till/x/repairs', true, false)).toBe(false)
  })
})
