import { describe, expect, it } from 'vitest'
import { askLinks } from './ask'

describe('askLinks', () => {
  it('turns the owner’s local number into WhatsApp and call links that name the install', () => {
    const a = askLinks('0769626396', 'C711564A')
    expect(a.tel).toBe('tel:+94769626396')
    expect(a.shown).toBe('076 962 6396')
    expect(a.whatsapp).toBe('https://wa.me/94769626396?text=' + encodeURIComponent('Hello Karots, I would like a code to try the Demo Room (install C711564A).'))
  })
})
