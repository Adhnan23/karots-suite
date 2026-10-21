import { describe, expect, it } from 'vitest'
import { failing, parseColour, ratio } from './contrast'

describe('contrast', () => {
  it('parses the forms getComputedStyle returns', () => {
    expect(parseColour('rgb(255, 0, 0)')).toEqual([255, 0, 0, 1])
    expect(parseColour('rgba(0, 0, 0, 0.5)')).toEqual([0, 0, 0, 0.5])
    expect(parseColour('color(srgb 1 0.5 0 / 0.25)')).toEqual([255, 127.5, 0, 0.25])
    expect(parseColour('nonsense')).toBeNull()
  })
  it('measures WCAG contrast', () => {
    expect(ratio([0, 0, 0, 1], [255, 255, 255, 1])).toBeCloseTo(21, 0)
    expect(ratio([119, 119, 119, 1], [255, 255, 255, 1])).toBeCloseTo(4.48, 1)
  })
  it('flags pairs under their minimum, with see-through layers composited on the page', () => {
    const colours: Record<string, string> = {
      '--background': 'rgb(255, 255, 255)',
      '--card': 'rgba(0, 0, 0, 0)', // fully see-through → the background shows
      '--foreground': 'rgb(200, 200, 200)', // too light on white
      '--muted-foreground': 'rgb(80, 80, 80)',
    }
    const bad = failing((t) => colours[t] ?? null, [
      { text: '--foreground', on: '--card', min: 4.5 },
      { text: '--muted-foreground', on: '--card', min: 4.5 },
    ])
    expect(bad.map((b) => b.text)).toEqual(['--foreground'])
    expect(bad[0].ratio).toBeLessThan(2)
  })
})
