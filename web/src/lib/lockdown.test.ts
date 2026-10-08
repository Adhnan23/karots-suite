import { describe, expect, it } from 'vitest'
import { lockFor } from './lockdown'

const k = (key: string, m: Partial<KeyboardEvent> = {}) => ({ key, ctrlKey: false, metaKey: false, shiftKey: false, altKey: false, ...m })

describe('lockFor', () => {
  it('F1 on a counter tab goes back to Sell; on Sell and admin it is just blocked', () => {
    expect(lockFor('/till/returns', k('F1'))).toBe('sell')
    expect(lockFor('/till', k('F1'))).toBe('block')
    expect(lockFor('/admin/products', k('F1'))).toBe('block')
    expect(lockFor('/admin', k('F5'))).toBe('block')
    expect(lockFor('/till', k('F12'))).toBe('block')
  })
  it('browser-chrome combos are blocked on the till only, editing keys never', () => {
    expect(lockFor('/till', k('r', { ctrlKey: true }))).toBe('block')
    expect(lockFor('/till/credit', k('P', { ctrlKey: true }))).toBe('block')
    expect(lockFor('/till', k('R', { ctrlKey: true, shiftKey: true }))).toBe(null)
    expect(lockFor('/till', k('v', { ctrlKey: true }))).toBe(null)
    expect(lockFor('/admin', k('r', { ctrlKey: true }))).toBe(null)
    expect(lockFor('/till', k('ArrowLeft', { altKey: true }))).toBe('block')
    expect(lockFor('/admin', k('ArrowLeft', { altKey: true }))).toBe(null)
  })
})
