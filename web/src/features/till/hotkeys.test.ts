import { describe, expect, it } from 'vitest'
import { hotkeyFor } from './hotkeys'

describe('till hotkeys', () => {
  const idle = { dialogOpen: false, saleDone: false }
  it('maps the karots-pos keys', () => {
    expect(hotkeyFor('F1', idle)).toBe('reset')
    expect(hotkeyFor('F2', idle)).toBe('search')
    expect(hotkeyFor('F3', idle)).toBe('search')
    expect(hotkeyFor('F4', idle)).toBe('hold')
    expect(hotkeyFor('F9', idle)).toBe('discount')
    expect(hotkeyFor('F10', idle)).toBe('pay')
    expect(hotkeyFor('Escape', idle)).toBe('blur')
    expect(hotkeyFor('Enter', idle)).toBeNull()
  })
  it('hold/pay/escape defer to an open dialog; Enter starts the next sale after completion', () => {
    const dlg = { dialogOpen: true, saleDone: false }
    expect(hotkeyFor('F4', dlg)).toBeNull()
    expect(hotkeyFor('F10', dlg)).toBeNull()
    expect(hotkeyFor('Escape', dlg)).toBeNull()
    expect(hotkeyFor('Enter', { dialogOpen: false, saleDone: true })).toBe('newSale')
  })
})
