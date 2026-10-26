import { BinaryBitmap, DecodeHintType, GlobalHistogramBinarizer, HybridBinarizer, MultiFormatOneDReader, QRCodeReader, RGBLuminanceSource } from '@zxing/library'
import QRCode from 'qrcode'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { BAR_QUIET, barModules, QR_QUIET } from '../codes'
import { paperOf, type PriceLabel } from '../kit'
import { LabelPage } from '../picture'
import { styleFor } from '../registry'
import { PRICE_KIND, PRICE_SAMPLE } from '.'

const EAN: PriceLabel = { ...PRICE_SAMPLE, code: '4792024012348', format: 'EAN13' }

/** The module widths a style drew, read back from its SVG (whole dots). */
function drawn(style: string, size: string, which: 'bars' | 'qr') {
  const el = document.createElement('div')
  el.innerHTML = renderToStaticMarkup(<LabelPage kind="price" data={EAN} paper={paperOf(size)} style={style} />)
  const svg = el.querySelector(`[data-code="${which}"]`)
  if (!svg) return { style: (el.firstElementChild as HTMLElement).dataset.labelStyle, module: 0 }
  const first = /h(\d+)v/.exec(svg.querySelector('path')!.getAttribute('d')!)!
  return { style: (el.firstElementChild as HTMLElement).dataset.labelStyle, module: Number(first[1]) }
}

const lum = (w: number, h: number, ink: (x: number, y: number) => boolean) => {
  const px = new Uint8ClampedArray(w * h)
  for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) px[y * w + x] = ink(x, y) ? 0 : 255
  return new RGBLuminanceSource(px, w, h)
}

function decodeBars(code: string, format: string, module: number) {
  const mods = barModules(code, format)!
  const w = (mods.length + 2 * BAR_QUIET) * module
  const src = lum(w, 8, (x) => mods[Math.floor(x / module) - BAR_QUIET] ?? false)
  return new MultiFormatOneDReader(new Map()).decode(new BinaryBitmap(new GlobalHistogramBinarizer(src))).getText()
}

function decodeQr(value: string, module: number) {
  const { modules } = QRCode.create(value, { errorCorrectionLevel: 'M' })
  const side = (modules.size + 2 * QR_QUIET) * module
  const at = (x: number, y: number) => {
    const mx = Math.floor(x / module) - QR_QUIET
    const my = Math.floor(y / module) - QR_QUIET
    return mx >= 0 && my >= 0 && mx < modules.size && my < modules.size && !!modules.get(my, mx)
  }
  const hints = new Map([[DecodeHintType.PURE_BARCODE, true]])
  return new QRCodeReader().decode(new BinaryBitmap(new HybridBinarizer(lum(side, side, at))), hints).getText()
}

describe('codes scan back', () => {
  for (const size of ['50x25', '38x25'])
    it(`QR Tag at ${size}: the QR decodes to the code, modules ≥ 2 dots`, () => {
      const q = drawn('qrtag', size, 'qr')
      expect(q.style).toBe('qrtag')
      expect(q.module).toBeGreaterThanOrEqual(2)
      expect(decodeQr(EAN.code, q.module)).toBe(EAN.code)
    })
  it('Dual at 50x25: both the EAN-13 and the QR decode', () => {
    const b = drawn('dual', '50x25', 'bars')
    const q = drawn('dual', '50x25', 'qr')
    expect(b.style).toBe('dual')
    expect(b.module).toBeGreaterThanOrEqual(2)
    expect(decodeBars(EAN.code, EAN.format, b.module)).toBe(EAN.code)
    expect(decodeQr(EAN.code, q.module)).toBe(EAN.code)
  })
  it('Dual at 38x25 is too small for both codes: it draws Classic, never a sub-minimum code', () => {
    expect(styleFor(PRICE_KIND, 'dual', paperOf('38x25'), EAN)).toMatchObject({ fellBack: true, style: { key: 'classic' } })
    expect(drawn('dual', '38x25', 'bars').style).toBe('classic')
  })
  it('every price style’s 1D bars decode at 50x25', () => {
    for (const s of ['classic', 'pricetag', 'inverse', 'ticket', 'minimal', 'dual']) {
      const b = drawn(s, '50x25', 'bars')
      expect(b.module, s).toBeGreaterThanOrEqual(2)
      expect(decodeBars(EAN.code, EAN.format, b.module), s).toBe(EAN.code)
    }
  })
})

describe('Shelf', () => {
  it('needs 70 mm: on a 50 mm sticker it draws as Price Tag', () => {
    expect(drawn('shelf', '50x25', 'bars').style).toBe('pricetag')
  })
  it('at 100x50: the hero, both codes and the date it was printed', () => {
    const el = document.createElement('div')
    el.innerHTML = renderToStaticMarkup(<LabelPage kind="price" data={EAN} paper={paperOf('100x50')} style="shelf" />)
    expect(el.querySelector('[data-label-style="shelf"]')).not.toBeNull()
    expect(el.querySelector('[data-hero]')?.textContent).toContain('1,250')
    expect(el.querySelector('[data-code="bars"]')).not.toBeNull()
    expect(el.querySelector('[data-code="qr"]')).not.toBeNull()
    expect(el.textContent).toMatch(/\d{1,2} [A-Z][a-z]{2} \d{4}/)
  })
})
