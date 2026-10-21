import { BarcodeFormat, BinaryBitmap, Code128Reader, DecodeHintType, GlobalHistogramBinarizer, RGBLuminanceSource } from '@zxing/library'
import { renderToStaticMarkup } from 'react-dom/server'
import { createElement } from 'react'
import { describe, expect, it } from 'vitest'
import { code128 } from './barcode'
import { Code128 } from './Barcode'

const NUMBERS = ['S-00231', 'CR-000018', 'DP-000002', 'R-0002', 's-77']

/** A 1-bit row as the printer draws it: quiet zone, bars at `module` dots, quiet zone. */
function row(widths: number[], module: number) {
  const quiet = 10 * module
  const px: number[] = Array(quiet).fill(255)
  widths.forEach((w, i) => px.push(...Array(w * module).fill(i % 2 === 0 ? 0 : 255)))
  px.push(...Array(quiet).fill(255))
  return px
}

function decode(px: number[], height = 4) {
  const lum = new Uint8ClampedArray(px.length * height)
  for (let y = 0; y < height; y++) lum.set(px, y * px.length)
  const bmp = new BinaryBitmap(new GlobalHistogramBinarizer(new RGBLuminanceSource(lum, px.length, height)))
  const hints = new Map([[DecodeHintType.POSSIBLE_FORMATS, [BarcodeFormat.CODE_128]]])
  return new Code128Reader().decode(bmp, hints).getText()
}

describe('code128', () => {
  it('starts with Start B and ends with the stop', () => {
    const w = code128('S-00231')
    expect(w.slice(0, 6)).toEqual([2, 1, 1, 2, 1, 4])
    expect(w.slice(-7)).toEqual([2, 3, 3, 1, 1, 1, 2])
    expect(w.reduce((a, b) => a + b, 0)).toBe(11 * ('S-00231'.length + 3) + 2)
  })
  for (const n of NUMBERS)
    it(`decodes back to ${n}`, () => {
      expect(decode(row(code128(n), 2))).toBe(n)
    })
  it('the drawn bars are whole dots at the module width', () => {
    const html = renderToStaticMarkup(createElement(Code128, { value: 'S-00231' }))
    const widths = [...html.matchAll(/width:(\d+)px/g)].map((m) => Number(m[1]))
    const bars = code128('S-00231').filter((_, i) => i % 2 === 0).map((w) => w * 2)
    expect(widths.slice(1)).toEqual(bars) // first width = the whole box
    expect(html).toContain('height:48px')
  })
})
