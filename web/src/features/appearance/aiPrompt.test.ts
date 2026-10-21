import { describe, expect, it } from 'vitest'
import { applyAiReply, buildPrompt } from './aiPrompt'
import { builtinDoc } from './paletteCss'

const studio = builtinDoc('studio')

describe('AI prompt', () => {
  it('asks for strict JSON and carries the current values', () => {
    const p = buildPrompt(studio)
    expect(p).toContain('"light"')
    expect(p).toContain('--brand')
    expect(p).toContain(studio.light['--brand'])
    expect(p).toMatch(/only the JSON/i)
    expect(p).toMatch(/--accent: HOVER background/) // a highlight colour there makes every hover loud
  })
})

describe('AI reply', () => {
  it('applies known tokens from a fenced JSON reply, both modes, and the name', () => {
    const reply = 'Here you go:\n```json\n{"name":"Sunset","light":{"--brand":"#E4572E","--nope":"1px"},"dark":{"--brand":"#FF8A65"}}\n```'
    const r = applyAiReply(studio, reply)
    expect(r.doc.light['--brand']).toBe('#E4572E')
    expect(r.doc.dark['--brand']).toBe('#FF8A65')
    expect(r.doc.light['--background']).toBe(studio.light['--background']) // untouched ones stay
    expect(r.doc.name).toBe('Sunset')
    expect(r.applied).toBe(2)
    expect(r.ignored).toEqual(['--nope'])
  })
  it('refuses values that could break out of the CSS', () => {
    const r = applyAiReply(studio, '{"light":{"--brand":"red; } body { display:none"}}')
    expect(r.doc.light['--brand']).toBe(studio.light['--brand'])
    expect(r.ignored).toEqual(['--brand'])
  })
  it('explains a reply that is not JSON', () => {
    expect(() => applyAiReply(studio, 'Sure! I love that idea.')).toThrow(/JSON/)
  })
})

describe('Surprise me', () => {
  it('lets the AI choose the look, with no description to fill in', () => {
    const p = buildPrompt(studio, { surprise: true })
    expect(p).toMatch(/surprise me/i)
    expect(p).not.toContain('<describe it here')
    expect(p).toMatch(/only the JSON/i)
  })
})
