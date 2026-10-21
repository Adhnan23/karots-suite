import { beforeEach, describe, expect, it } from 'vitest'
import { applyAppearance } from './appearance'

const base = { palette: 'studio', brandColor: '', radius: 'rounded', receiptStyle: 'classic' } as const

describe('applyAppearance', () => {
  beforeEach(() => localStorage.clear())

  it('sets brand and radius, no density', () => {
    const el = document.createElement('div')
    applyAppearance({ ...base, brandColor: '#0f766e', radius: 'round' }, el)
    expect(el.style.getPropertyValue('--brand')).toBe('#0f766e')
    expect(el.style.getPropertyValue('--radius')).toBe('22px')
    expect(el.dataset.ui).toBe('dock')
    expect(el.dataset.density).toBeUndefined()
  })
  it('picks readable text for a custom brand', () => {
    const el = document.createElement('div')
    applyAppearance({ ...base, brandColor: '#0f766e' }, el)
    expect(el.style.getPropertyValue('--brand-foreground')).toBe('#ffffff')
    applyAppearance({ ...base, brandColor: '#ffe066' }, el)
    expect(el.style.getPropertyValue('--brand-foreground')).toBe('#15161a')
  })
  it('clears a previous brand when reset to theme default', () => {
    const el = document.createElement('div')
    applyAppearance({ ...base, brandColor: '#0f766e' }, el)
    applyAppearance({ ...base, brandColor: '' }, el)
    expect(el.style.getPropertyValue('--brand')).toBe('')
    expect(el.style.getPropertyValue('--brand-foreground')).toBe('')
  })
  it('sets data-palette and defaults to studio', () => {
    const el = document.createElement('div')
    applyAppearance({ ...base, palette: 'night' }, el)
    expect(el.dataset.palette).toBe('night')
    applyAppearance({ ...base, palette: '' }, el)
    expect(el.dataset.palette).toBe('studio')
  })
  it('caches the palette in localStorage as ks-palette', () => {
    const el = document.createElement('div')
    applyAppearance({ ...base, palette: 'night' }, el)
    expect(localStorage.getItem('ks-palette')).toBe('night')
  })
  it('legacy radius applies to a built-in only', () => {
    const el = document.createElement('div')
    applyAppearance({ ...base, radius: 'sharp' }, el)
    expect(el.style.getPropertyValue('--radius')).toBe('6px')
    applyAppearance({ ...base, radius: 'sharp', palette: 'custom-3' }, el)
    expect(el.style.getPropertyValue('--radius')).toBe('')
  })
})
