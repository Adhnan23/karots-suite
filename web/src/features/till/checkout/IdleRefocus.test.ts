import { expect, it } from 'vitest'
import { IDLE_MS, shouldRefocus } from './IdleRefocus'

it('puts the cursor back in search after 20 s idle, never over a dialog or when it is already there', () => {
  expect(shouldRefocus(IDLE_MS, false, 'till-amount')).toBe(true)
  expect(shouldRefocus(IDLE_MS - 1, false, 'till-amount')).toBe(false)
  expect(shouldRefocus(IDLE_MS, true, 'till-amount')).toBe(false)
  expect(shouldRefocus(IDLE_MS, false, 'till-search')).toBe(false)
  expect(shouldRefocus(IDLE_MS, false, undefined)).toBe(true)
})
