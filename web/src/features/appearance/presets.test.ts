import { describe, expect, it } from 'vitest'
import { builtinDoc } from './paletteCss'
import { applyMaterial, applyMotion, materialOf, motionOf } from './presets'

const studio = builtinDoc('studio')

describe('material presets', () => {
  it('liquid glass blurs and goes translucent; solid goes opaque', () => {
    const liquid = applyMaterial(studio.light, 'liquid', 'light')
    expect(liquid['--glass']).toMatch(/blur/)
    expect(liquid['--card']).toMatch(/rgb\(.* \/ \.\d+\)/)
    expect(applyMaterial(studio.dark, 'solid', 'dark')['--glass']).toBe('none')
  })
  it('reads the material back', () => {
    for (const m of ['solid', 'frosted', 'liquid'] as const) expect(materialOf(applyMaterial(studio.light, m, 'light'))).toBe(m)
    expect(materialOf(builtinDoc('night').light)).toBe('frosted')
    expect(materialOf(builtinDoc('lagoon').light)).toBe('liquid')
  })
})

describe('motion presets', () => {
  it('lively lifts and overshoots, speed scales, slow stays ≤ 350ms', () => {
    const t = applyMotion(studio.light, 'lively', 1.5)
    expect(t['--lift']).toBe('2px')
    expect(t['--dur-fast']).toBe('225ms')
    expect(t['--dur-slow']).toBe('350ms')
    expect(motionOf(t)).toEqual({ feel: 'lively', speed: 1.5 })
  })
  it('calm at normal speed is Studio’s motion', () => {
    const t = applyMotion(studio.light, 'calm', 1)
    for (const k of ['--dur-fast', '--dur', '--dur-slow', '--ease', '--lift']) expect(t[k]).toBe(studio.light[k])
    expect(motionOf(studio.light)).toEqual({ feel: 'calm', speed: 1 })
  })
})

describe('depth', () => {
  it('keeps the glass rim and swaps the drop shadow', async () => {
    const { withDrop } = await import('./EditorControls')
    const lagoon = 'inset 0 1px 0 rgb(255 255 255 / .9), inset 0 0 0 1px rgb(255 255 255 / .35), 0 8px 30px rgb(10 60 70 / .10)'
    expect(withDrop(lagoon, '0 1px 2px rgb(0 0 0 / .05)')).toBe('inset 0 1px 0 rgb(255 255 255 / .9), inset 0 0 0 1px rgb(255 255 255 / .35), 0 1px 2px rgb(0 0 0 / .05)')
    expect(withDrop('0 8px 24px rgb(0 0 0 / .08)', 'X')).toBe('X')
  })
})
