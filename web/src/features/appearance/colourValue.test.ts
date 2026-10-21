import { describe, expect, it } from 'vitest'
import { formatColour, parseCssColour } from './colourValue'

describe('colour values', () => {
  it('reads hex and rgb forms', () => {
    expect(parseCssColour('#6E5BFF')).toEqual({ hex: '#6E5BFF', alpha: 1 })
    expect(parseCssColour('#abc')).toEqual({ hex: '#AABBCC', alpha: 1 })
    expect(parseCssColour('rgb(255 255 255 / .06)')).toEqual({ hex: '#FFFFFF', alpha: 0.06 })
    expect(parseCssColour('rgba(14, 120, 130, 0.08)')).toEqual({ hex: '#0E7882', alpha: 0.08 })
    expect(parseCssColour('color(srgb 1 0 0.5 / 0.5)')).toEqual({ hex: '#FF0080', alpha: 0.5 })
    expect(parseCssColour('var(--foreground)')).toBeNull()
  })
  it('writes hex when solid, rgb with alpha when see-through', () => {
    expect(formatColour('#FF0000', 1)).toBe('#FF0000')
    expect(formatColour('#FFFFFF', 0.06)).toBe('rgb(255 255 255 / .06)')
    expect(formatColour('#0E7882', 0.5)).toBe('rgb(14 120 130 / .5)')
  })
})
