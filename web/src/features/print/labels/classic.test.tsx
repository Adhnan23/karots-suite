import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { barModules, fitModule } from './codes'
import { paperOf, type PriceLabel } from './kit'
import { LabelPage } from './picture'
import { PRICE_SAMPLE } from './price'
import { styleFor } from './registry'
import { PRICE_KIND } from './price'

const html = (d: PriceLabel, size = '50x25', style = 'classic') => {
  const el = document.createElement('div')
  el.innerHTML = renderToStaticMarkup(<LabelPage kind="price" data={d} paper={paperOf(size)} style={style} />)
  return el.firstElementChild as HTMLElement
}

describe('paperOf', () => {
  it('reads WxH mm at 8 dots per mm, clamped like the server', () => {
    expect(paperOf('50x25')).toEqual({ wMm: 50, hMm: 25, w: 400, h: 200 })
    expect(paperOf('38 x 25')).toMatchObject({ w: 304, h: 200 })
    expect(paperOf('', '40x30')).toMatchObject({ wMm: 40, hMm: 30 })
    expect(paperOf('5x900')).toMatchObject({ wMm: 10, hMm: 200 })
  })
})

describe('Classic price label', () => {
  it('is exactly the paper size and draws first text, bars + digits, second text in that order', () => {
    const root = html(PRICE_SAMPLE)
    expect(root.style.width).toBe('400px')
    expect(root.style.height).toBe('200px')
    expect(root.dataset.labelStyle).toBe('classic')
    const order = Array.from(root.querySelectorAll('[data-slot],[data-code],[data-digits]'), (e) => (e as HTMLElement).dataset.slot ?? (e as HTMLElement).dataset.code ?? 'digits')
    expect(order).toEqual(['name', 'bars', 'digits', 'price'])
    expect(root.textContent).toContain('Anchor Milk Powder 400g')
    expect(root.textContent).toContain('Rs 1,250.00')
  })
  it('a slot set to none is not drawn', () => {
    const root = html({ ...PRICE_SAMPLE, second: { key: 'none', text: '' } })
    expect(root.querySelector('[data-slot="price"]')).toBeNull()
    expect(root.querySelectorAll('[data-slot]')).toHaveLength(1)
  })
  it('wraps a text to its line count (the picture reports a cut when it overflows)', () => {
    const root = html({ ...PRICE_SAMPLE, firstLines: 3 })
    expect((root.querySelector('[data-slot="name"]') as HTMLElement).style.webkitLineClamp).toBe('3')
  })
  it('a code not valid for its format draws no bars', () => {
    expect(html({ ...PRICE_SAMPLE, code: 'ABC', format: 'EAN13' }).querySelector('[data-code]')).toBeNull()
  })
})

describe('codes', () => {
  it('EAN-13 is 95 modules; a module is never thinner than 2 dots', () => {
    expect(barModules('4792024012348', 'EAN13')).toHaveLength(95)
    expect(fitModule(95, 10, 368, 2)).toBe(2)
    expect(fitModule(95, 10, 200)).toBe(0)
  })
})

describe('styleFor', () => {
  it('an unknown style draws classic', () => {
    expect(styleFor(PRICE_KIND, 'nope', paperOf('50x25'), PRICE_SAMPLE).style.key).toBe('classic')
  })
})
