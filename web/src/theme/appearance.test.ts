import { describe, expect, it } from 'vitest'
import { applyAppearance } from './appearance'

describe('applyAppearance', () => {
  it('sets brand, radius and density', () => {
    const el = document.createElement('div')
    applyAppearance({ brandColor: '#0f766e', radius: 'round', density: 'compact', receiptStyle: 'classic' }, el)
    expect(el.style.getPropertyValue('--brand')).toBe('#0f766e')
    expect(el.dataset.radius).toBe('round')
    expect(el.dataset.density).toBe('compact')
  })
  it('picks readable text for a custom brand', () => {
    const el = document.createElement('div')
    applyAppearance({ brandColor: '#0f766e', radius: 'rounded', density: 'comfortable', receiptStyle: 'classic' }, el)
    expect(el.style.getPropertyValue('--brand-foreground')).toBe('#ffffff')
    applyAppearance({ brandColor: '#ffe066', radius: 'rounded', density: 'comfortable', receiptStyle: 'classic' }, el)
    expect(el.style.getPropertyValue('--brand-foreground')).toBe('#15161a')
  })
  it('clears a previous brand when reset to theme default', () => {
    const el = document.createElement('div')
    applyAppearance({ brandColor: '#0f766e', radius: 'rounded', density: 'comfortable', receiptStyle: 'classic' }, el)
    applyAppearance({ brandColor: '', radius: 'rounded', density: 'comfortable', receiptStyle: 'classic' }, el)
    expect(el.style.getPropertyValue('--brand')).toBe('')
    expect(el.style.getPropertyValue('--brand-foreground')).toBe('')
  })
})
