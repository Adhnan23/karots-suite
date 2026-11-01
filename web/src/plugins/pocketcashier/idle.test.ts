import { renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useIdle } from './idle'

describe('the phone locks when idle', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('after the shop\'s minutes with no touch; a touch starts the wait again', () => {
    const onIdle = vi.fn()
    renderHook(() => useIdle(2, onIdle))
    vi.advanceTimersByTime(90_000)
    window.dispatchEvent(new Event('touchstart'))
    vi.advanceTimersByTime(90_000)
    expect(onIdle).not.toHaveBeenCalled()
    vi.advanceTimersByTime(31_000)
    expect(onIdle).toHaveBeenCalledTimes(1)
  })

  it('never with 0 minutes', () => {
    const onIdle = vi.fn()
    renderHook(() => useIdle(0, onIdle))
    vi.advanceTimersByTime(24 * 3600_000)
    expect(onIdle).not.toHaveBeenCalled()
  })
})
