import { describe, expect, it } from 'vitest'
import { toOneBit } from './raster'

const px = (...rgba: number[]) => new Uint8ClampedArray(rgba)
const INK = [0, 0, 0, 255]
const PAPER = [255, 255, 255, 255]

describe('toOneBit', () => {
  it('turns each pixel into ink or paper at luminance 160', () => {
    const p = px(200, 200, 200, 255, 150, 150, 150, 255, 160, 160, 160, 255)
    toOneBit(p)
    expect(Array.from(p)).toEqual([...PAPER, ...INK, ...PAPER])
  })
  it('a transparent pixel is paper', () => {
    const p = px(0, 0, 0, 0)
    toOneBit(p)
    expect(Array.from(p)).toEqual(PAPER)
  })
  it('weighs the channels by luminance', () => {
    const p = px(255, 0, 0, 255, 0, 0, 255, 255, 255, 255, 0, 255) // red ≈ 76, blue ≈ 29, yellow ≈ 226
    toOneBit(p)
    expect(Array.from(p)).toEqual([...INK, ...INK, ...PAPER])
  })
})
